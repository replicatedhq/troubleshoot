package redact

import (
	"bytes"
	"io"
	"regexp"
)

// lineState represents a line and whether it ended with a newline character.
// This is used by MultiLineRedactor to track newline state for line pairs.
type lineState struct {
	content    []byte
	hadNewline bool
}

type MultiLineRedactor struct {
	scan       *regexp.Regexp
	re1        *regexp.Regexp
	re2        *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

func NewMultiLineRedactor(re1 LineRedactor, re2 string, maskText, path, name string, isDefault bool) (*MultiLineRedactor, error) {
	var scanCompiled *regexp.Regexp
	compiled1, err := compileRegex(re1.regex)
	if err != nil {
		return nil, err
	}

	if re1.scan != "" {
		scanCompiled, err = compileRegex(re1.scan)
		if err != nil {
			return nil, err
		}
	}

	compiled2, err := compileRegex(re2)
	if err != nil {
		return nil, err
	}

	return &MultiLineRedactor{scan: scanCompiled, re1: compiled1, re2: compiled2, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
}

// Redact processes the input reader in pairs of lines, applying redaction patterns.
// Unlike the previous implementation using bufio.Reader with readLine(), this now
// uses LineReader to preserve the exact newline structure of the input file.
//
// The MultiLineRedactor works by:
// 1. Reading pairs of lines (line1, line2)
// 2. If line1 matches the selector pattern (re1), redact line2 using re2
// 3. Write both lines with their original newline structure preserved
//
// This ensures binary files and text files without trailing newlines are not corrupted.
func (r *MultiLineRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()
	go func() {
		var err error
		defer func() {
			writer.CloseWithError(err)
		}()

		tokenizer := GetGlobalTokenizer()
		lineReader := NewLineReader(input)

		// Try to read first two lines
		line1, nl1, line2, nl2, readErr := getNextTwoLines(lineReader, nil)

		// Handle case where we can't read 2 lines (empty file or single line)
		// Note: We check line1 == nil (not len(line1) == 0) because:
		// - nil means truly empty file with no content
		// - []byte{} (len==0) means an empty line that had a newline (e.g., "\n")
		if readErr != nil && line1 == nil {
			// Empty file - nothing to write
			return
		}

		if readErr != nil {
			// Only 1 line available (or empty line with newline) - write it and exit
			// FIX: This is the bug fix - only add newline if original had one
			// Also handles empty lines (line1 == []byte{} with nl1 == true)
			err = writeLine(writer, line1, nl1)
			if err != nil {
				return
			}
			return
		}

		// Process line pairs
		flushLastLine := false
		lineNum := 1

		for readErr == nil {
			lineNum++ // the first line that can be redacted is line 2

			// Pre-filter: if scan is not nil, check if line1 matches scan by lowercasing it
			if r.scan != nil {
				lowerLine1 := bytes.ToLower(line1)
				if !r.scan.Match(lowerLine1) {
					// No match - write line1 and advance
					err = writeLine(writer, line1, nl1)
					if err != nil {
						return
					}
					line1, nl1, line2, nl2, readErr = getNextTwoLines(lineReader, &lineState{line2, nl2})
					flushLastLine = true
					continue
				}
			}

			// Check if line1 matches the selector pattern (re1)
			if !r.re1.Match(line1) {
				// No match - write line1 and advance
				err = writeLine(writer, line1, nl1)
				if err != nil {
					return
				}
				line1, nl1, line2, nl2, readErr = getNextTwoLines(lineReader, &lineState{line2, nl2})
				flushLastLine = true
				continue
			}

			// line1 matched selector - redact line2
			flushLastLine = false
			var clean []byte
			if tokenizer.IsEnabled() {
				// Use tokenized replacement for line2 based on line1 context
				context := r.redactName
				clean = getTokenizedReplacementPatternWithPath(r.re2, line2, context, r.filePath)
			} else {
				// Use original masking behavior
				substStr := []byte(getReplacementPattern(r.re2, r.maskText))
				clean = r.re2.ReplaceAll(line2, substStr)
			}

			// Write line1 (selector line) and line2 (redacted line)
			err = writeLine(writer, line1, nl1)
			if err != nil {
				return
			}
			err = writeLine(writer, clean, nl2)
			if err != nil {
				return
			}

			// Track redaction if content changed
			if !bytes.Equal(clean, line2) {
				addRedaction(Redaction{
					RedactorName:      r.redactName,
					CharactersRemoved: len(line2) - len(clean),
					Line:              lineNum,
					File:              r.filePath,
					IsDefaultRedactor: r.isDefault,
				})
			}

			// Get next pair
			line1, nl1, line2, nl2, readErr = getNextTwoLines(lineReader, nil)
		}

		// After loop exits (readErr != nil), check if we have an unwritten line1
		// This happens in two cases:
		// 1. flushLastLine=true: line1 was advanced but not written (scan/re1 didn't match)
		// 2. len(line1) > 0: we read line1 but couldn't get line2 (unpaired line at end)
		if flushLastLine || len(line1) > 0 {
			err = writeLine(writer, line1, nl1)
			if err != nil {
				return
			}
		}

		// Propagate non-EOF read errors to the caller
		// EOF is expected (end of file) and not an error condition
		// Note: readErr is always non-nil here (loop exited), but we only propagate non-EOF errors
		if readErr != io.EOF {
			err = readErr
		}
	}()
	return out
}

// getNextTwoLines reads the next pair of lines from the LineReader.
// It returns the content and newline state for both lines.
//
// If curLine2 is provided, it's used as line1 (optimization for advancing through file).
// Otherwise, both lines are read fresh from the reader.
//
// Returns:
//   - line1, hadNewline1: First line content and newline state
//   - line2, hadNewline2: Second line content and newline state
//   - err: Error only if we couldn't read line1, or if line2 read failed with non-EOF error
//
// Note: If line2 returns (content, false, io.EOF), we treat this as SUCCESS because
// we got the content. The EOF just means it didn't have a trailing newline.
func getNextTwoLines(lr *LineReader, curLine2 *lineState) (
	line1 []byte, hadNewline1 bool,
	line2 []byte, hadNewline2 bool,
	err error,
) {
	if curLine2 == nil {
		// Read both lines fresh
		line1, hadNewline1, err = lr.ReadLine()
		if err != nil {
			return
		}

		line2, hadNewline2, err = lr.ReadLine()
		// If we got line2 content but hit EOF, that's OK - it just means no trailing newline
		if err == io.EOF && len(line2) > 0 {
			err = nil // Clear the error - we successfully read both lines
		}
		return
	}

	// Use cached line2 as new line1 (optimization)
	line1 = curLine2.content
	hadNewline1 = curLine2.hadNewline

	// Read new line2
	line2, hadNewline2, err = lr.ReadLine()
	// If we got line2 content but hit EOF, that's OK - it just means no trailing newline
	if err == io.EOF && len(line2) > 0 {
		err = nil // Clear the error - we successfully read both lines
	}
	return
}

// writeLine writes a line to the writer, optionally adding a newline if hadNewline is true.
// This helper reduces code duplication in the Redact function.
func writeLine(w io.Writer, line []byte, hadNewline bool) error {
	if err := writeBytes(w, line); err != nil {
		return err
	}
	if hadNewline {
		if err := writeBytes(w, NEW_LINE); err != nil {
			return err
		}
	}
	return nil
}

// writeBytes writes all byte slices to the writer
// in the order they are passed in the variadic argument
func writeBytes(w io.Writer, bs ...[]byte) error {
	for _, b := range bs {
		_, err := w.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}

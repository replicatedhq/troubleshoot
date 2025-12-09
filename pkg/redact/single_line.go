package redact

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"k8s.io/klog/v2"
)

type SingleLineRedactor struct {
	scan       *regexp.Regexp
	re         *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

var NEW_LINE = []byte{'\n'}

func NewSingleLineRedactor(re LineRedactor, maskText, path, name string, isDefault bool) (*SingleLineRedactor, error) {
	var scanCompiled *regexp.Regexp
	compiled, err := compileRegex(re.regex)
	if err != nil {
		return nil, err
	}

	if re.scan != "" {
		scanCompiled, err = compileRegex(re.scan)
		if err != nil {
			return nil, err
		}
	}

	return &SingleLineRedactor{scan: scanCompiled, re: compiled, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
}

// Redact processes the input reader line-by-line, applying redaction patterns.

// Unlike the previous implementation using bufio.Scanner, this now uses LineReader
// to preserve the exact newline structure of the input file. Lines that originally
// ended with \n will have \n added back, while lines without \n (like the last line
// of a file without a trailing newline, or binary files) will not have \n added.
// This ensures binary files and text files without trailing newlines are not corrupted.
func (r *SingleLineRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()

	go func() {
		var err error
		defer func() {
			if err == nil || err == io.EOF {
				writer.Close()
			} else {
				// Check if error is about line exceeding maximum size
				if err != nil && bytes.Contains([]byte(err.Error()), []byte("exceeds maximum size")) {
					s := fmt.Sprintf("Error redacting %q. A line in the file exceeded %d MB max length", path, constants.SCANNER_MAX_SIZE/1024/1024)
					klog.V(2).Info(s)
				} else {
					klog.V(2).Info(fmt.Sprintf("Error redacting %q: %v", path, err))
				}
				writer.CloseWithError(err)
			}
		}()

		// Use LineReader instead of bufio.Scanner to track newline presence
		lineReader := NewLineReader(input)
		tokenizer := GetGlobalTokenizer()
		lineNum := 0

		for {
			line, hadNewline, readErr := lineReader.ReadLine()

			// Handle EOF with no content - we're done
			if readErr == io.EOF && len(line) == 0 {
				break
			}

			// We have content to process
			lineNum++

			// Determine if we should redact this line
			shouldRedact := true

			// Pre-filter: if scan is not nil, check if line matches scan by lowercasing it
			if r.scan != nil {
				lowerLine := bytes.ToLower(line)
				if !r.scan.Match(lowerLine) {
					shouldRedact = false
				}
			}

			// Check if line matches the main redaction pattern
			if shouldRedact && !r.re.Match(line) {
				shouldRedact = false
			}

			// Process the line (redact or pass through)
			var outputLine []byte
			if shouldRedact {
				// Line matches - perform redaction
				if tokenizer.IsEnabled() {
					// Use tokenized replacement - context comes from the redactor name
					context := r.redactName
					outputLine = getTokenizedReplacementPatternWithPath(r.re, line, context, r.filePath)
				} else {
					// Use original masking behavior
					substStr := []byte(getReplacementPattern(r.re, r.maskText))
					outputLine = r.re.ReplaceAll(line, substStr)
				}

				// Track redaction if content changed
				if !bytes.Equal(outputLine, line) {
					addRedaction(Redaction{
						RedactorName:      r.redactName,
						CharactersRemoved: len(line) - len(outputLine),
						Line:              lineNum,
						File:              r.filePath,
						IsDefaultRedactor: r.isDefault,
					})
				}
			} else {
				// No match - use original line
				outputLine = line
			}

			// Write the line
			err = writeBytes(writer, outputLine)
			if err != nil {
				return
			}
			// Only add newline if original line had one
			if hadNewline {
				err = writeBytes(writer, NEW_LINE)
				if err != nil {
					return
				}
			}

			// Check if we hit EOF after processing this line
			if readErr == io.EOF {
				break
			}
			// Check for non-EOF errors
			if readErr != nil {
				err = readErr
				return
			}
		}
	}()
	return out
}

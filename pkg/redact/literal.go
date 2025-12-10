package redact

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"k8s.io/klog/v2"
)

type literalRedactor struct {
	match      []byte
	filePath   string
	redactName string
	isDefault  bool
}

func literalString(match []byte, path, name string) Redactor {
	return literalRedactor{
		match:      match,
		filePath:   path,
		redactName: name,
	}
}

// Redact processes the input reader line-by-line, replacing literal string matches.
// Unlike the previous implementation using bufio.Scanner, this now uses LineReader
// to preserve the exact newline structure of the input file. Lines that originally
// ended with \n will have \n added back, while lines without \n (like the last line
// of a file without a trailing newline, or binary files) will not have \n added.
// This ensures binary files and text files without trailing newlines are not corrupted.
func (r literalRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()

	go func() {
		var err error
		defer func() {
			if err == nil || err == io.EOF {
				writer.Close()
			} else {
				// Check if error is about line exceeding maximum size
				if errors.Is(err, bufio.ErrTooLong) {
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

			// Perform literal string replacement
			var clean []byte
			if tokenizer.IsEnabled() {
				// For literal redaction, we tokenize the matched value
				matchStr := string(r.match)
				context := r.redactName
				token := tokenizer.TokenizeValueWithPath(matchStr, context, r.filePath)
				clean = bytes.ReplaceAll(line, r.match, []byte(token))
			} else {
				// Use original masking behavior
				clean = bytes.ReplaceAll(line, r.match, maskTextBytes)
			}

			// Write the line (redacted or original)
			err = writeBytes(writer, clean)
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

			// Track redaction if content changed
			if !bytes.Equal(clean, line) {
				addRedaction(Redaction{
					RedactorName:      r.redactName,
					CharactersRemoved: len(line) - len(clean),
					Line:              lineNum,
					File:              r.filePath,
					IsDefaultRedactor: r.isDefault,
				})
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

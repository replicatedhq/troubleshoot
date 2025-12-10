package redact

import (
	"bufio"
	"io"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

// LineReader reads lines from an io.Reader while tracking whether each line
// ended with a newline character. This is essential for preserving the exact
// structure of input files during redaction - binary files and text files
// without trailing newlines should not have newlines added to them.
//
// Unlike bufio.Scanner which strips newlines and requires the caller to add
// them back, LineReader explicitly tracks the presence of newlines so callers
// can conditionally restore them only when they were originally present.
type LineReader struct {
	reader *bufio.Reader
}

// NewLineReader creates a new LineReader that reads from the given io.Reader.
// The reader is wrapped in a bufio.Reader for efficient byte-by-byte reading.
func NewLineReader(r io.Reader) *LineReader {
	return &LineReader{
		reader: bufio.NewReader(r),
	}
}

// ReadLine reads the next line from the reader and returns:
//   - line content (without the newline character if present)
//   - whether the line ended with a newline (\n)
//   - any error encountered
//
// Return values:
//   - (content, true, nil)      - line ended with \n, more content may follow
//   - (content, false, io.EOF)  - last line without \n (file doesn't end with newline)
//   - (nil, false, io.EOF)      - reached EOF with no content (empty file or end of file)
//   - (content, false, error)   - encountered a non-EOF error
//
// The function respects constants.SCANNER_MAX_SIZE and returns an error if a single
// line exceeds this limit. This prevents memory exhaustion on files with extremely
// long lines or binary files without newlines that are larger than the limit.
//
// Example usage:
//
//	lr := NewLineReader(input)
//	for {
//	    line, hadNewline, err := lr.ReadLine()
//	    if err == io.EOF && len(line) == 0 {
//	        break // End of file, no more content
//	    }
//
//	    // Process line...
//	    fmt.Print(string(line))
//	    if hadNewline {
//	        fmt.Print("\n")
//	    }
//
//	    if err == io.EOF {
//	        break // Last line processed
//	    }
//	    if err != nil {
//	        return err
//	    }
//	}
func (lr *LineReader) ReadLine() ([]byte, bool, error) {
	// Initialize line as empty slice (not nil) to ensure consistent return values
	// Empty lines (just \n) should return []byte{}, not nil
	line := []byte{}

	for {
		b, err := lr.reader.ReadByte()

		// Handle errors
		if err == io.EOF {
			if len(line) > 0 {
				// Last line without newline - return the content we have
				return line, false, io.EOF
			}
			// Nothing left to read - empty file or end of content
			return nil, false, io.EOF
		}
		if err != nil {
			// Non-EOF error encountered
			return line, false, err
		}

		// Found newline character
		if b == '\n' {
			// Return the line (may be empty for blank lines)
			return line, true, nil
		}

		// Accumulate byte into line buffer
		line = append(line, b)

		// Check buffer limit to prevent memory exhaustion
		// This is especially important for binary files without newlines
		if len(line) > constants.SCANNER_MAX_SIZE {
			return nil, false, bufio.ErrTooLong
		}
	}
}

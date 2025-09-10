package redact

import (
	"bufio"
	"bytes"
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

func (r literalRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()

	go func() {
		var err error
		defer func() {
			if err == nil || err == io.EOF {
				writer.Close()
			} else {
				if err == bufio.ErrTooLong {
					s := fmt.Sprintf("Error redacting %q. A line in the file exceeded %d MB max length", path, constants.SCANNER_MAX_SIZE/1024/1024)
					klog.V(2).Info(s)
				} else {
					klog.V(2).Info(fmt.Sprintf("Error redacting %q: %v", path, err))
				}
				writer.CloseWithError(err)
			}
		}()

		buf := make([]byte, constants.BUF_INIT_SIZE)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(buf, constants.SCANNER_MAX_SIZE)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			replacement := maskTextBytes
			if enableTokenization {
				tok := tokenizeValue(r.match, inferTypeHint(r.redactName))
				replacement = []byte(tok)
			}
			clean := bytes.ReplaceAll(line, r.match, replacement)

			// Append newline since scanner strips it
			err = writeBytes(writer, clean, NEW_LINE)
			if err != nil {
				return
			}

			if !bytes.Equal(clean, line) {
				addRedaction(Redaction{
					RedactorName:      r.redactName,
					CharactersRemoved: len(line) - len(clean),
					Line:              lineNum,
					File:              r.filePath,
					IsDefaultRedactor: r.isDefault,
				})
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			err = scanErr
		}
	}()
	return out
}

package redact

import (
	"bufio"
	"bytes"
	"io"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
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
			if err == io.EOF {
				writer.Close()
			} else {
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

			clean := bytes.ReplaceAll(line, r.match, maskTextBytes)

			_, err = writer.Write(append(clean, '\n')) // Append newline since scanner strips it
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

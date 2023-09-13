package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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

		mask := []byte(MASK_TEXT)

		reader := bufio.NewReader(input)
		lineNum := 0
		for {
			lineNum++
			var line []byte
			line, err = readLine(reader)
			if err != nil {
				return
			}

			clean := bytes.ReplaceAll(line, r.match, mask)

			// io.WriteString would be nicer, but scanner strips new lines
			fmt.Fprintf(writer, "%s\n", clean)
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
	}()
	return out
}

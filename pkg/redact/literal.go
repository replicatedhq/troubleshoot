package redact

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type literalRedactor struct {
	matchString string
	filePath    string
}

func literalString(matchString, path string) Redactor {
	return literalRedactor{
		matchString: matchString,
		filePath:    path,
	}
}

func (r literalRedactor) Redact(input io.Reader) io.Reader {
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

		reader := bufio.NewReader(input)
		lineNum := 0
		for {
			lineNum++
			var line string
			line, err = readLine(reader)
			if err != nil {
				return
			}

			clean := strings.ReplaceAll(line, r.matchString, MASK_TEXT)

			// io.WriteString would be nicer, but scanner strips new lines
			fmt.Fprintf(writer, "%s\n", clean)
			if err != nil {
				return
			}

			if clean != line {
				go addRedaction(Redaction{
					RedactorName:      fmt.Sprintf("literal %q", r.matchString),
					CharactersRemoved: len(line) - len(clean),
					Line:              lineNum,
					File:              r.filePath,
				})
			}
		}
	}()
	return out
}

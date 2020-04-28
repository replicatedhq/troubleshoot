package redact

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type literalRedactor struct {
	matchString string
}

func literalString(matchString string) Redactor {
	return literalRedactor{matchString: matchString}
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
		for {
			var line string
			line, err = readLine(reader)
			if err != nil {
				return
			}

			// io.WriteString would be nicer, but scanner strips new lines
			fmt.Fprintf(writer, "%s\n", strings.ReplaceAll(line, r.matchString, MASK_TEXT))
			if err != nil {
				return
			}
		}
	}()
	return out
}

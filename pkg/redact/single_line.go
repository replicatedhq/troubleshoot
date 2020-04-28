package redact

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
)

type SingleLineRedactor struct {
	re       *regexp.Regexp
	maskText string
}

func NewSingleLineRedactor(re, maskText string) (*SingleLineRedactor, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return nil, err
	}
	return &SingleLineRedactor{re: compiled, maskText: maskText}, nil
}

func (r *SingleLineRedactor) Redact(input io.Reader) io.Reader {
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

		substStr := getReplacementPattern(r.re, r.maskText)

		reader := bufio.NewReader(input)
		for {
			var line string
			line, err = readLine(reader)
			if err != nil {
				return
			}

			if !r.re.MatchString(line) {
				fmt.Fprintf(writer, "%s\n", line)
				continue
			}

			clean := r.re.ReplaceAllString(line, substStr)

			// io.WriteString would be nicer, but scanner strips new lines
			fmt.Fprintf(writer, "%s\n", clean)
			if err != nil {
				return
			}
		}
	}()
	return out
}

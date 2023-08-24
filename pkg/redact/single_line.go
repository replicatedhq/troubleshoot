package redact

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type SingleLineRedactor struct {
	scan       *regexp.Regexp
	re         *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

func NewSingleLineRedactor(re LineRedactor, maskText, path, name string, isDefault bool) (*SingleLineRedactor, error) {
	var scanCompiled *regexp.Regexp
	compiled, err := regexp.Compile(re.regex)
	if err != nil {
		return nil, err
	}

	if re.scan != "" {
		scanCompiled, err = regexp.Compile(re.scan)
		if err != nil {
			return nil, err
		}
	}

	return &SingleLineRedactor{scan: scanCompiled, re: compiled, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
}

func (r *SingleLineRedactor) Redact(input io.Reader, path string) io.Reader {
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

		buf := make([]byte, constants.MAX_BUFFER_CAPACITY)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(buf, constants.MAX_BUFFER_CAPACITY)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// is scan is not nil, then check if line matches scan by lowercasing it
			if r.scan != nil {
				lowerLine := strings.ToLower(line)
				if !r.scan.MatchString(lowerLine) {
					fmt.Fprintf(writer, "%s\n", line)
					continue
				}
			}

			// if scan matches, but re does not, do not redact
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

			// if clean is not equal to line, a redaction was performed
			if clean != line {
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

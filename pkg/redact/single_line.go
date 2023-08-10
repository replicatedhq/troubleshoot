package redact

import (
	"bufio"
	"io"
	"regexp"
	"strings"
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
	bufferedWriter := bufio.NewWriter(writer)

	go func() {
		var err error
		defer func() {
			bufferedWriter.Flush()
			if err == io.EOF {
				writer.Close()
			} else {
				writer.CloseWithError(err)
			}
		}()

		substStr := getReplacementPattern(r.re, r.maskText)

		scanner := bufio.NewScanner(input)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			lowerLine := strings.ToLower(line)
			// if scan is not nil, do not redact if the line does not match
			if r.scan != nil && !r.scan.MatchString(lowerLine) {
				bufferedWriter.WriteString(line)
				bufferedWriter.WriteByte('\n')
				continue
			}

			// if scan matches, but re does not, do not redact
			if !r.re.MatchString(line) {
				bufferedWriter.WriteString(line)
				bufferedWriter.WriteByte('\n')
				continue
			}

			clean := r.re.ReplaceAllString(line, substStr)

			// io.WriteString would be nicer, but scanner strips new lines
			bufferedWriter.WriteString(clean)
			bufferedWriter.WriteByte('\n')
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

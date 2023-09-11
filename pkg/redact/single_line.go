package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
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
	compiled, err := compileRegex(re.regex)
	if err != nil {
		return nil, err
	}

	if re.scan != "" {
		scanCompiled, err = compileRegex(re.scan)
		if err != nil {
			return nil, err
		}
	}

	return &SingleLineRedactor{scan: scanCompiled, re: compiled, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
}

const (
	// This is the initial size of the buffer allocated.
	// Under the hood, an array of size N is allocated in memory
	BUF_INIT_SIZE = 4096 // 4KB

	// This is the muximum size the buffer can grow to
	SCANNER_MAX_SIZE = 1024 * 1024 // 1MB
)

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

		substStr := []byte(getReplacementPattern(r.re, r.maskText))

		buf := make([]byte, BUF_INIT_SIZE)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(buf, SCANNER_MAX_SIZE)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			// is scan is not nil, then check if line matches scan by lowercasing it
			if r.scan != nil {
				lowerLine := bytes.ToLower(line)
				if !r.scan.Match(lowerLine) {
					fmt.Fprintf(writer, "%s\n", line)
					continue
				}
			}

			// if scan matches, but re does not, do not redact
			if !r.re.Match(line) {
				fmt.Fprintf(writer, "%s\n", line)
				continue
			}

			clean := r.re.ReplaceAll(line, substStr)

			// io.WriteString would be nicer, but scanner strips new lines
			fmt.Fprintf(writer, "%s\n", clean)

			if err != nil {
				return
			}

			// if clean is not equal to line, a redaction was performed
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

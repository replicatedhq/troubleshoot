package redact

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

type SingleLineRedactor struct {
	re         *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

func NewSingleLineRedactor(re, maskText, path, bundlePath string, name string, isDefault bool) (*SingleLineRedactor, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filepath.Join(bundlePath, path))
	if err != nil {
		return nil, err
	}

	if !compiled.MatchString(string(content)) {
		fmt.Printf("No matches found for %s in %s\n", re, path)
		return nil, nil
	}

	return &SingleLineRedactor{re: compiled, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
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

		const maxCapacity = 1024 * 1024
		buf := make([]byte, maxCapacity)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(buf, maxCapacity)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if !r.re.MatchString(line) {
				bufferedWriter.WriteString(line)
				bufferedWriter.WriteByte('\n')
				continue
			}

			clean := r.re.ReplaceAllString(line, substStr)
			fmt.Println("clean: ", clean)
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

package redact

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
)

type MultiLineRedactor struct {
	scan       *regexp.Regexp
	re1        *regexp.Regexp
	re2        *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

func NewMultiLineRedactor(re1 LineRedactor, re2 string, maskText, path, name string, isDefault bool) (*MultiLineRedactor, error) {
	var scanCompiled *regexp.Regexp
	compiled1, err := compileRegex(re1.regex)
	if err != nil {
		return nil, err
	}

	if re1.scan != "" {
		scanCompiled, err = compileRegex(re1.scan)
		if err != nil {
			return nil, err
		}
	}

	compiled2, err := compileRegex(re2)
	if err != nil {
		return nil, err
	}

	return &MultiLineRedactor{scan: scanCompiled, re1: compiled1, re2: compiled2, maskText: maskText, filePath: path, redactName: name, isDefault: isDefault}, nil
}

func (r *MultiLineRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()
	go func() {
		var err error
		defer func() {
			writer.CloseWithError(err)
		}()

		substStr := []byte(getReplacementPattern(r.re2, r.maskText))

		reader := bufio.NewReader(input)
		line1, line2, err := getNextTwoLines(reader, nil)
		if err != nil {
			// this will print 2 blank lines for empty input...
			// Append newlines since scanner strips them
			err = writeBytes(writer, line1, NEW_LINE, line2, NEW_LINE)
			if err != nil {
				return
			}

			return
		}

		flushLastLine := false
		lineNum := 1
		for err == nil {
			lineNum++ // the first line that can be redacted is line 2

			// is scan is not nil, then check if line1 matches scan by lowercasing it
			if r.scan != nil {
				lowerLine1 := bytes.ToLower(line1)
				if !r.scan.Match(lowerLine1) {
					// Append newline since scanner strips it
					err = writeBytes(writer, line1, NEW_LINE)
					if err != nil {
						return
					}
					line1, line2, err = getNextTwoLines(reader, &line2)
					flushLastLine = true
					continue
				}
			}

			// If line1 matches re1, then transform line2 using re2
			if !r.re1.Match(line1) {
				// Append newline since scanner strips it
				err = writeBytes(writer, line1, NEW_LINE)
				if err != nil {
					return
				}
				line1, line2, err = getNextTwoLines(reader, &line2)
				flushLastLine = true
				continue
			}
			flushLastLine = false
			clean := r.re2.ReplaceAll(line2, substStr)
			if enableTokenization && !bytes.Equal(clean, line2) {
				// Find the "mask" named group for tokenization
				if matches := r.re2.FindSubmatch(line2); matches != nil {
					// Find the mask group index
					maskGroupIndex := -1
					for i, name := range r.re2.SubexpNames() {
						if name == "mask" {
							maskGroupIndex = i
							break
						}
					}
					if maskGroupIndex > 0 && maskGroupIndex < len(matches) && len(matches[maskGroupIndex]) > 0 {
						token := tokenizeValue(matches[maskGroupIndex], inferTypeHint(r.redactName))
						clean = bytes.ReplaceAll(clean, []byte(r.maskText), []byte(token))
					}
				}
			}

			// Append newlines since scanner strips them
			err = writeBytes(writer, line1, NEW_LINE, clean, NEW_LINE)
			if err != nil {
				return
			}

			// if clean is not equal to line2, a redaction was performed
			if !bytes.Equal(clean, line2) {
				addRedaction(Redaction{
					RedactorName:      r.redactName,
					CharactersRemoved: len(line2) - len(clean),
					Line:              lineNum,
					File:              r.filePath,
					IsDefaultRedactor: r.isDefault,
				})
			}

			line1, line2, err = getNextTwoLines(reader, nil)
		}

		if flushLastLine {
			// Append newline since scanner strip it
			err = writeBytes(writer, line1, NEW_LINE)
			if err != nil {
				return
			}
		}
	}()
	return out
}

func getNextTwoLines(reader *bufio.Reader, curLine2 *[]byte) (line1 []byte, line2 []byte, err error) {
	line2 = []byte{}

	if curLine2 == nil {
		line1, err = readLine(reader)
		if err != nil {
			return
		}

		line2, err = readLine(reader)
		return
	}

	line1 = *curLine2
	line2, err = readLine(reader)
	if err != nil {
		return
	}

	return
}

// writeBytes writes all byte slices to the writer
// in the order they are passed in the variadic argument
func writeBytes(w io.Writer, bs ...[]byte) error {
	for _, b := range bs {
		_, err := w.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}

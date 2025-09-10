package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"k8s.io/klog/v2"
)

type SingleLineRedactor struct {
	scan       *regexp.Regexp
	re         *regexp.Regexp
	maskText   string
	filePath   string
	redactName string
	isDefault  bool
}

var NEW_LINE = []byte{'\n'}

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

func (r *SingleLineRedactor) Redact(input io.Reader, path string) io.Reader {
	out, writer := io.Pipe()

	go func() {
		var err error
		defer func() {
			if err == nil || err == io.EOF {
				writer.Close()
			} else {
				if err == bufio.ErrTooLong {
					s := fmt.Sprintf("Error redacting %q. A line in the file exceeded %d MB max length", path, constants.SCANNER_MAX_SIZE/1024/1024)
					klog.V(2).Info(s)
				} else {
					klog.V(2).Info(fmt.Sprintf("Error redacting %q: %v", path, err))
				}
				writer.CloseWithError(err)
			}
		}()

		substStr := []byte(getReplacementPattern(r.re, r.maskText))

		buf := make([]byte, constants.BUF_INIT_SIZE)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(buf, constants.SCANNER_MAX_SIZE)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			// is scan is not nil, then check if line matches scan by lowercasing it
			if r.scan != nil {
				lowerLine := bytes.ToLower(line)
				if !r.scan.Match(lowerLine) {
					// Append newline since scanner strips it
					err = writeBytes(writer, line, NEW_LINE)
					if err != nil {
						return
					}
					continue
				}
			}

			// if scan matches, but re does not, do not redact
			if !r.re.Match(line) {
				// Append newline since scanner strips it
				err = writeBytes(writer, line, NEW_LINE)
				if err != nil {
					return
				}
				continue
			}

			clean := r.re.ReplaceAll(line, substStr)
			if enableTokenization && !bytes.Equal(clean, line) {
				// Find the "mask" named group for tokenization
				if matches := r.re.FindSubmatch(line); matches != nil {
					// Find the mask group index
					maskGroupIndex := -1
					for i, name := range r.re.SubexpNames() {
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
			// Append newline since scanner strips it
			err = writeBytes(writer, clean, NEW_LINE)
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

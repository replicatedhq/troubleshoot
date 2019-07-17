package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
)

const (
	MASK_TEXT = "***HIDDEN***"
)

type Redactor interface {
	Redact(input io.Reader) io.Reader
}

func Redact(input []byte) ([]byte, error) {
	redactors, err := GetRedactors()
	if err != nil {
		return nil, err
	}

	nextReader := io.Reader(bytes.NewReader(input))
	for _, r := range redactors {
		nextReader = r.Redact(nextReader)
	}

	redacted, err := ioutil.ReadAll(nextReader)
	if err != nil {
		return nil, err
	}

	return redacted, nil
}

func GetRedactors() ([]Redactor, error) {
	// TODO: Make this configurable

	// (?i) makes it case insensitive
	// groups named with `?P<mask>` will be masked
	// groups named with `?P<drop>` will be removed (replaced with empty strings)
	singleLines := []string{
		// ipv4
		`(?P<mask>\b(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b)`,
		// TODO: ipv6
		// aws secrets
		`(?i)(\\\"name\\\":\\\"[^\"]*SECRET_?ACCESS_?KEY\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		`(?i)(\\\"name\\\":\\\"[^\"]*ACCESS_?KEY_?ID\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		`(?i)(\\\"name\\\":\\\"[^\"]*OWNER_?ACCOUNT\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		// passwords in general
		`(?i)(\\\"name\\\":\\\"[^\"]*password[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		// tokens in general
		`(?i)(\\\"name\\\":\\\"[^\"]*token[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		`(?i)(\\\"name\\\":\\\"[^\"]*database[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		`(?i)(\\\"name\\\":\\\"[^\"]*user[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
		// connection strings with username and password
		// http://user:password@host:8888
		`(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
		// user:password@tcp(host:3309)/db-name
		`\b(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@tcp\()(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\)\/)(?P<mask>[\w\d\S-_]+){1}\b`,
		// standard postgres and mysql connnection strings
		`(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(location *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(User ID *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(password *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(Server *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(Database *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(Uid *= *)(?P<mask>[^\;]+)(;)`,
		`(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
	}

	redactors := make([]Redactor, 0)
	for _, re := range singleLines {
		r, err := NewSingleLineRedactor(re, MASK_TEXT)
		if err != nil {
			return nil, err // maybe skip broken ones?
		}
		redactors = append(redactors, r)
	}

	doubleLines := []struct {
		line1 string
		line2 string
	}{
		{
			line1: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *"[^\"]*OWNER_?ACCOUNT[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *".*password[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *".*token[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *".*database[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
		{
			line1: `(?i)"name": *".*user[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
		},
	}

	for _, l := range doubleLines {
		r, err := NewMultiLineRedactor(l.line1, l.line2, MASK_TEXT)
		if err != nil {
			return nil, err // maybe skip broken ones?
		}
		redactors = append(redactors, r)
	}

	return redactors, nil
}

func getReplacementPattern(re *regexp.Regexp, maskText string) string {
	substStr := ""
	for i, name := range re.SubexpNames() {
		if i == 0 { // index 0 is the entire string
			continue
		}
		if name == "" {
			substStr = fmt.Sprintf("%s$%d", substStr, i)
		} else if name == "mask" {
			substStr = fmt.Sprintf("%s%s", substStr, maskText)
		} else if name == "drop" {
			// no-op, string is just dropped from result
		} else {
			substStr = fmt.Sprintf("%s${%s}", substStr, name)
		}
	}
	return substStr
}

func readLine(r *bufio.Reader) (string, error) {
	var completeLine []byte
	for {
		var line []byte
		line, isPrefix, err := r.ReadLine()
		if err != nil {
			return "", err
		}

		completeLine = append(completeLine, line...)
		if !isPrefix {
			break
		}
	}
	return string(completeLine), nil
}

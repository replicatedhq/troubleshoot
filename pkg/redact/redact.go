package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"sync"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

const (
	MASK_TEXT = "***HIDDEN***"
)

var allRedactions RedactionList
var redactionListMut sync.Mutex
var pendingRedactions sync.WaitGroup

func init() {
	allRedactions = RedactionList{
		ByRedactor: map[string][]Redaction{},
		ByFile:     map[string][]Redaction{},
	}
}

type Redactor interface {
	Redact(input io.Reader) io.Reader
}

// Redactions are indexed both by the file affected and by the name of the redactor
type RedactionList struct {
	ByRedactor map[string][]Redaction `json:"byRedactor" yaml:"byRedactor"`
	ByFile     map[string][]Redaction `json:"byFile" yaml:"byFile"`
}

type Redaction struct {
	RedactorName      string `json:"redactorName" yaml:"redactorName"`
	CharactersRemoved int    `json:"charactersRemoved" yaml:"charactersRemoved"`
	Line              int    `json:"line" yaml:"line"`
	File              string `json:"file" yaml:"file"`
}

func Redact(input []byte, path string, additionalRedactors []*troubleshootv1beta1.Redact) ([]byte, error) {
	redactors, err := getRedactors(path)
	if err != nil {
		return nil, err
	}

	builtRedactors, err := buildAdditionalRedactors(path, additionalRedactors)
	if err != nil {
		return nil, errors.Wrap(err, "build custom redactors")
	}
	redactors = append(redactors, builtRedactors...)

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

func GetRedactionList() RedactionList {
	pendingRedactions.Wait()
	redactionListMut.Lock()
	defer redactionListMut.Unlock()
	return allRedactions
}

func ResetRedactionList() {
	redactionListMut.Lock()
	defer redactionListMut.Unlock()
	allRedactions = RedactionList{
		ByRedactor: map[string][]Redaction{},
		ByFile:     map[string][]Redaction{},
	}
}

func buildAdditionalRedactors(path string, redacts []*troubleshootv1beta1.Redact) ([]Redactor, error) {
	additionalRedactors := []Redactor{}
	for i, redact := range redacts {
		if redact == nil {
			continue
		}

		// check if redact matches path
		matches, err := redactMatchesPath(path, redact)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}

		withinRedactNum := 0 // give unique redaction names

		for _, re := range redact.Regex {
			r, err := NewSingleLineRedactor(re, MASK_TEXT, path, redactorName(i, withinRedactNum, redact.Name, "regex"))
			if err != nil {
				return nil, errors.Wrapf(err, "redactor %q", re)
			}
			additionalRedactors = append(additionalRedactors, r)
			withinRedactNum++
		}

		for _, literal := range redact.Values {
			additionalRedactors = append(additionalRedactors, literalString(literal, path, redactorName(i, withinRedactNum, redact.Name, "literal")))
			withinRedactNum++
		}

		for _, re := range redact.MultiLine {
			r, err := NewMultiLineRedactor(re.Selector, re.Redactor, MASK_TEXT, path, redactorName(i, withinRedactNum, redact.Name, "multiLine"))
			if err != nil {
				return nil, errors.Wrapf(err, "multiline redactor %+v", re)
			}
			additionalRedactors = append(additionalRedactors, r)
			withinRedactNum++
		}

		for _, yaml := range redact.Yaml {
			r := NewYamlRedactor(yaml, path, redactorName(i, withinRedactNum, redact.Name, "yaml"))
			additionalRedactors = append(additionalRedactors, r)
			withinRedactNum++
		}
	}
	return additionalRedactors, nil
}

func redactMatchesPath(path string, redact *troubleshootv1beta1.Redact) (bool, error) {
	if redact.File == "" && len(redact.Files) == 0 {
		return true, nil
	}

	globs := []glob.Glob{}

	if redact.File != "" {
		newGlob, err := glob.Compile(redact.File, '/')
		if err != nil {
			return false, errors.Wrapf(err, "invalid file glob string %q", redact.File)
		}
		globs = append(globs, newGlob)
	}

	for i, fileGlobString := range redact.Files {
		newGlob, err := glob.Compile(fileGlobString, '/')
		if err != nil {
			return false, errors.Wrapf(err, "invalid file glob string %d %q", i, fileGlobString)
		}
		globs = append(globs, newGlob)
	}

	for _, thisGlob := range globs {
		if thisGlob.Match(path) {
			return true, nil
		}
	}

	return false, nil
}

func getRedactors(path string) ([]Redactor, error) {
	// TODO: Make this configurable

	// (?i) makes it case insensitive
	// groups named with `?P<mask>` will be masked
	// groups named with `?P<drop>` will be removed (replaced with empty strings)
	singleLines := []struct {
		regex string
		name  string
	}{
		// ipv4
		{
			regex: `(?P<mask>\b(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b)`,
			name:  "ipv4",
		},
		// TODO: ipv6
		// aws secrets
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*SECRET_?ACCESS_?KEY\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "SECRET_ACCESS_KEY",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*ACCESS_?KEY_?ID\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "ACCESS_KEY_ID",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*OWNER_?ACCOUNT\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "OWNER_ACCOUNT",
		},
		// passwords in general
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*password[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "password",
		},
		// tokens in general
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*token[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "token",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*database[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "database",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*user[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "user",
		},
		// connection strings with username and password
		// http://user:password@host:8888
		{
			regex: `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
			name:  "http://user:password@host:8888",
		},
		// user:password@tcp(host:3309)/db-name
		{
			regex: `\b(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@tcp\()(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\)\/)(?P<mask>[\w\d\S-_]+){1}\b`,
			name:  "user:password@tcp(host:3309)/db-name",
		},
		// standard postgres and mysql connection strings
		{
			regex: `(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Data Source",
		},
		{
			regex: `(?i)(location *= *)(?P<mask>[^\;]+)(;)`,
			name:  "location",
		},
		{
			regex: `(?i)(User ID *= *)(?P<mask>[^\;]+)(;)`,
			name:  "User ID",
		},
		{
			regex: `(?i)(password *= *)(?P<mask>[^\;]+)(;)`,
			name:  "db-password",
		},
		{
			regex: `(?i)(Server *= *)(?P<mask>[^\;]+)(;)`,
			name:  "server",
		},
		{
			regex: `(?i)(Database *= *)(?P<mask>[^\;]+)(;)`,
			name:  "db-database",
		},
		{
			regex: `(?i)(Uid *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Uid",
		},
		{
			regex: `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Pwd",
		},
	}

	redactors := make([]Redactor, 0)
	for _, re := range singleLines {
		r, err := NewSingleLineRedactor(re.regex, MASK_TEXT, path, redactorName(-1, -1, re.name, "defaultRegex"))
		if err != nil {
			return nil, err // maybe skip broken ones?
		}
		redactors = append(redactors, r)
	}

	doubleLines := []struct {
		line1 string
		line2 string
		name  string
	}{
		{
			line1: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "SECRET_ACCESS_KEY",
		},
		{
			line1: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "ACCESS_KEY_ID",
		},
		{
			line1: `(?i)"name": *"[^\"]*OWNER_?ACCOUNT[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "OWNER_ACCOUNT",
		},
		{
			line1: `(?i)"name": *".*password[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "password",
		},
		{
			line1: `(?i)"name": *".*token[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "token",
		},
		{
			line1: `(?i)"name": *".*database[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "database",
		},
		{
			line1: `(?i)"name": *".*user[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "user",
		},
	}

	for _, l := range doubleLines {
		r, err := NewMultiLineRedactor(l.line1, l.line2, MASK_TEXT, path, redactorName(-1, -1, l.name, "defaultMultiLine"))
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

func addRedaction(redaction Redaction) {
	pendingRedactions.Add(1)
	go func(redaction Redaction) {
		redactionListMut.Lock()
		defer redactionListMut.Unlock()
		defer pendingRedactions.Done()
		allRedactions.ByRedactor[redaction.RedactorName] = append(allRedactions.ByRedactor[redaction.RedactorName], redaction)
		allRedactions.ByFile[redaction.File] = append(allRedactions.ByFile[redaction.File], redaction)
	}(redaction)
}

func redactorName(redactorNum, withinRedactorNum int, redactorName, redactorType string) string {
	if withinRedactorNum == -1 {
		return fmt.Sprintf("%s.%q", redactorType, redactorName)
	}
	if redactorName != "" {
		return fmt.Sprintf("%s-%d", redactorName, withinRedactorNum)
	}
	return fmt.Sprintf("unnamed-%d.%d-%s", redactorNum, withinRedactorNum, redactorType)
}

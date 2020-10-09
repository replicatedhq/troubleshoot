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
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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
	IsDefaultRedactor bool   `json:"isDefaultRedactor" yaml:"isDefaultRedactor"`
}

func Redact(input []byte, path string, additionalRedactors []*troubleshootv1beta2.Redact) ([]byte, error) {
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

func buildAdditionalRedactors(path string, redacts []*troubleshootv1beta2.Redact) ([]Redactor, error) {
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

		for j, literal := range redact.Removals.Values {
			additionalRedactors = append(additionalRedactors, literalString(literal, path, redactorName(i, j, redact.Name, "literal")))
		}

		for j, re := range redact.Removals.Regex {
			var newRedactor Redactor
			if re.Selector != "" {
				newRedactor, err = NewMultiLineRedactor(re.Selector, re.Redactor, MASK_TEXT, path, redactorName(i, j, redact.Name, "multiLine"), false)
				if err != nil {
					return nil, errors.Wrapf(err, "multiline redactor %+v", re)
				}
			} else {
				newRedactor, err = NewSingleLineRedactor(re.Redactor, MASK_TEXT, path, redactorName(i, j, redact.Name, "regex"), false)
				if err != nil {
					return nil, errors.Wrapf(err, "redactor %q", re)
				}
			}
			additionalRedactors = append(additionalRedactors, newRedactor)
		}

		for j, yaml := range redact.Removals.YamlPath {
			r := NewYamlRedactor(yaml, path, redactorName(i, j, redact.Name, "yaml"))
			additionalRedactors = append(additionalRedactors, r)
		}
	}
	return additionalRedactors, nil
}

func redactMatchesPath(path string, redact *troubleshootv1beta2.Redact) (bool, error) {
	if redact.FileSelector.File == "" && len(redact.FileSelector.Files) == 0 {
		return true, nil
	}

	globs := []glob.Glob{}

	if redact.FileSelector.File != "" {
		newGlob, err := glob.Compile(redact.FileSelector.File, '/')
		if err != nil {
			return false, errors.Wrapf(err, "invalid file glob string %q", redact.FileSelector.File)
		}
		globs = append(globs, newGlob)
	}

	for i, fileGlobString := range redact.FileSelector.Files {
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
		//{
		//regex: `(?P<mask>\b(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b)`,
		//name:  "Redact ipv4 addresses",
		//},
		// TODO: ipv6
		// aws secrets
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*SECRET_?ACCESS_?KEY\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables that look like AWS Secret Access Keys",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*ACCESS_?KEY_?ID\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables that look like AWS Access Keys",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*OWNER_?ACCOUNT\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables that look like AWS Owner or Account numbers",
		},
		// passwords in general
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*password[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables with names beginning with 'password'",
		},
		// tokens in general
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*token[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables with names beginning with 'token'",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*database[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables with names beginning with 'database'",
		},
		{
			regex: `(?i)(\\\"name\\\":\\\"[^\"]*user[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
			name:  "Redact values for environment variables with names beginning with 'user'",
		},
		// connection strings with username and password
		// http://user:password@host:8888
		{
			regex: `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
			name:  "Redact connection strings with username and password",
		},
		// user:password@tcp(host:3309)/db-name
		{
			regex: `\b(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@tcp\()(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\)\/)(?P<mask>[\w\d\S-_]+){1}\b`,
			name:  "Redact database connection strings that contain username and password",
		},
		// standard postgres and mysql connection strings
		{
			regex: `(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'Data Source' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(location *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'location' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(User ID *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'User ID' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(password *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'password' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(Server *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'Server' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(Database *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'Database' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(Uid *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'UID' values commonly found in database connection strings",
		},
		{
			regex: `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			name:  "Redact 'Pwd' values commonly found in database connection strings",
		},
	}

	redactors := make([]Redactor, 0)
	for _, re := range singleLines {
		r, err := NewSingleLineRedactor(re.regex, MASK_TEXT, path, re.name, true)
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
			name:  "Redact AWS Secret Access Key values in multiline JSON",
		},
		{
			line1: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact AWS Access Key ID values in multiline JSON",
		},
		{
			line1: `(?i)"name": *"[^\"]*OWNER_?ACCOUNT[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact AWS Owner and Account Numbers in multiline JSON",
		},
		{
			line1: `(?i)"name": *".*password[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact password environment variables in multiline JSON",
		},
		{
			line1: `(?i)"name": *".*token[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact values that look like API tokens in multiline JSON",
		},
		{
			line1: `(?i)"name": *".*database[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact database connection strings in multiline JSON",
		},
		{
			line1: `(?i)"name": *".*user[^\"]*"`,
			line2: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:  "Redact usernames in multiline JSON",
		},
	}

	for _, l := range doubleLines {
		r, err := NewMultiLineRedactor(l.line1, l.line2, MASK_TEXT, path, l.name, true)
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
	if redactorName != "" {
		return fmt.Sprintf("%s.%s.%d", redactorName, redactorType, withinRedactorNum)
	}
	return fmt.Sprintf("unnamed-%d.%s.%d", redactorNum, redactorType, withinRedactorNum)
}

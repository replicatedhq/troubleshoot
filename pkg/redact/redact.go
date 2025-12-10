package redact

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

const (
	MASK_TEXT = "***HIDDEN***"
)

var (
	allRedactions     RedactionList
	redactionListMut  sync.Mutex
	pendingRedactions sync.WaitGroup

	// A regex cache to avoid recompiling the same regexes over and over
	regexCache     = map[string]*regexp.Regexp{}
	regexCacheLock sync.Mutex
	maskTextBytes  = []byte(MASK_TEXT)
)

func init() {
	allRedactions = RedactionList{
		ByRedactor: map[string][]Redaction{},
		ByFile:     map[string][]Redaction{},
	}
}

// A regex cache to avoid recompiling the same regexes over and over
func compileRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheLock.Lock()
	defer regexCacheLock.Unlock()

	if cached, ok := regexCache[pattern]; ok {
		return cached, nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	regexCache[pattern] = compiled
	return compiled, nil
}

type Redactor interface {
	Redact(input io.Reader, path string) io.Reader
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

type LineRedactor struct {
	regex string
	scan  string
}

func Redact(input io.Reader, path string, additionalRedactors []*troubleshootv1beta2.Redact) (io.Reader, error) {
	redactors, err := getRedactors(path)
	if err != nil {
		return nil, err
	}

	builtRedactors, err := buildAdditionalRedactors(path, additionalRedactors)
	if err != nil {
		return nil, errors.Wrap(err, "build custom redactors")
	}
	redactors = append(redactors, builtRedactors...)

	nextReader := input
	for _, r := range redactors {
		nextReader = r.Redact(nextReader, path)
	}

	return nextReader, nil
}

func GetRedactionList() RedactionList {
	pendingRedactions.Wait()
	redactionListMut.Lock()
	defer redactionListMut.Unlock()
	return allRedactions
}

func ResetRedactionList() {
	// Wait for all pending redaction goroutines to complete before resetting
	// This prevents race conditions where goroutines write to the map after reset
	pendingRedactions.Wait()
	
	redactionListMut.Lock()
	defer redactionListMut.Unlock()
	allRedactions = RedactionList{
		ByRedactor: map[string][]Redaction{},
		ByFile:     map[string][]Redaction{},
	}

	// Clear the regex cache as well. We do not want
	// to keep this around in long running processes
	// that continually redact files
	regexCacheLock.Lock()
	defer regexCacheLock.Unlock()

	regexCache = map[string]*regexp.Regexp{}
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
			additionalRedactors = append(additionalRedactors, literalString([]byte(literal), path, redactorName(i, j, redact.Name, "literal")))
		}

		for j, re := range redact.Removals.Regex {
			var newRedactor Redactor
			if re.Selector != "" {
				newRedactor, err = NewMultiLineRedactor(LineRedactor{
					regex: re.Selector,
				}, re.Redactor, MASK_TEXT, path, redactorName(i, j, redact.Name, "multiLine"), false)
				if err != nil {
					return nil, errors.Wrapf(err, "multiline redactor %+v", re)
				}
			} else {
				newRedactor, err = NewSingleLineRedactor(LineRedactor{
					regex: re.Redactor,
				}, MASK_TEXT, path, redactorName(i, j, redact.Name, "regex"), false)
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
		regex LineRedactor
		name  string
	}{
		// aws secrets
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*SECRET_?ACCESS_?KEY\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `secret_?access_?key`,
			},
			name: "Redact values for environment variables that look like AWS Secret Access Keys",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*ACCESS_?KEY_?ID\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `access_?key_?id`,
			},
			name: "Redact values for environment variables that look like AWS Access Keys",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*OWNER_?ACCOUNT\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `owner_?account`,
			},
			name: "Redact values for environment variables that look like AWS Owner or Account numbers",
		},
		// passwords in general
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*password[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `password`,
			},
			name: "Redact values for environment variables with names beginning with 'password'",
		},
		// tokens in general
		{

			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*token[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `token`,
			},
			name: "Redact values for environment variables with names beginning with 'token'",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*database[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `database`,
			},
			name: "Redact values for environment variables with names beginning with 'database'",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\\\"name\\\":\\\"[^\"]*user[^\"]*\\\",\\\"value\\\":\\\")(?P<mask>[^\"]*)(\\\")`,
				scan:  `user`,
			},
			name: "Redact values for environment variables with names beginning with 'user'",
		},
		// connection strings with username and password
		// http://user:password@host:8888
		{
			regex: LineRedactor{
				regex: `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
				scan:  `https?|ftp`,
			},
			name: "Redact connection strings with username and password",
		},
		// user:password@tcp(host:3309)/db-name
		{
			regex: LineRedactor{
				regex: `\b(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@tcp\()(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\)\/)(?P<mask>[\w\d\S-_]+){1}\b`,
				scan:  `@tcp`,
			},
			name: "Redact database connection strings that contain username and password",
		},
		// standard postgres and mysql connection strings
		// protocol://user:password@host:5432/db
		{
			regex: LineRedactor{
				regex: `\b(\w*:\/\/)(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@)(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\/)(?P<mask>[\w\d\S-_]+){1}\b`,
				scan:  `\b(\w*:\/\/)([^:\"\/]*)(:)([^@\"\/]*)(@)([^:\"\/]*)(:[\d]*)?(\/)([\w\d\S-_]+)\b`,
			},
			name: "Redact database connection strings that contain username and password",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `data source`,
			},
			name: "Redact 'Data Source' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(location *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `location`,
			},
			name: "Redact 'location' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(User ID *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `user id`,
			},
			name: "Redact 'User ID' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(password *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `password`,
			},
			name: "Redact 'password' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(Server *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `server`,
			},
			name: "Redact 'Server' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(Database *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `database`,
			},
			name: "Redact 'Database' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(Uid *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `uid`,
			},
			name: "Redact 'UID' values commonly found in database connection strings",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
				scan:  `pwd`,
			},
			name: "Redact 'Pwd' values commonly found in database connection strings",
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
		selector LineRedactor
		redactor string
		name     string
	}{
		{
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
				scan:  `secret_?access_?key`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact AWS Secret Access Key values in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
				scan:  `access_?key_?id`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact AWS Access Key ID values in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*OWNER_?ACCOUNT[^\"]*"`,
				scan:  `owner_?account`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact AWS Owner and Account Numbers in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *".*password[^\"]*"`,
				scan:  `password`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact password environment variables in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *".*token[^\"]*"`,
				scan:  `token`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact values that look like API tokens in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *".*database[^\"]*"`,
				scan:  `database`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact database connection strings in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"name": *".*user[^\"]*"`,
				scan:  `user`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			name:     "Redact usernames in multiline JSON",
		},
		{
			selector: LineRedactor{
				regex: `(?i)"entity": *"(osd|client|mgr)\..*[^\"]*"`,
				scan:  `(osd|client|mgr)`,
			},
			redactor: `(?i)("key": *")(?P<mask>.{38}==[^\"]*)(")`,
			name:     "Redact 'key' values found in Ceph auth lists",
		},
	}

	for _, l := range doubleLines {
		r, err := NewMultiLineRedactor(l.selector, l.redactor, MASK_TEXT, path, l.name, true)
		if err != nil {
			return nil, err // maybe skip broken ones?
		}
		redactors = append(redactors, r)
	}

	customResources := []struct {
		resource string
		yamlPath string
	}{
		{
			resource: "installers.cluster.kurl.sh",
			yamlPath: "*.spec.kubernetes.bootstrapToken",
		},
		{
			resource: "installers.cluster.kurl.sh",
			yamlPath: "*.spec.kubernetes.certKey",
		},
		{
			resource: "installers.cluster.kurl.sh",
			yamlPath: "*.spec.kubernetes.kubeadmToken",
		},
	}

	uniqueCRs := map[string]bool{}
	for _, cr := range customResources {
		fileglob := fmt.Sprintf("%s/%s/%s/*.yaml", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_CUSTOM_RESOURCES, cr.resource)
		redactors = append(redactors, NewYamlRedactor(cr.yamlPath, fileglob, ""))

		// redact kubectl last applied annotation once for each resource since it contains copies of
		// redacted fields
		if !uniqueCRs[cr.resource] {
			uniqueCRs[cr.resource] = true
			redactors = append(redactors, &YamlRedactor{
				filePath: fileglob,
				maskPath: []string{"*", "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
			})
		}
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

// getTokenizedReplacementPattern creates a replacement pattern that tokenizes matched groups
func getTokenizedReplacementPattern(re *regexp.Regexp, line []byte, context string) []byte {
	return getTokenizedReplacementPatternWithPath(re, line, context, "")
}

// getTokenizedReplacementPatternWithPath creates a replacement pattern that tokenizes matched groups with file path tracking
func getTokenizedReplacementPatternWithPath(re *regexp.Regexp, line []byte, context, filePath string) []byte {
	tokenizer := GetGlobalTokenizer()
	if !tokenizer.IsEnabled() {
		// Fallback to original behavior
		return []byte(getReplacementPattern(re, MASK_TEXT))
	}

	// Find all matches and their submatches
	matches := re.FindSubmatch(line)
	if matches == nil {
		return line // No match found
	}

	substStr := ""
	for i, name := range re.SubexpNames() {
		if i == 0 { // index 0 is the entire string
			continue
		}
		if i >= len(matches) {
			continue
		}

		if name == "" {
			// Unnamed group - preserve as is
			substStr = fmt.Sprintf("%s$%d", substStr, i)
		} else if name == "mask" {
			// This is the group to be tokenized
			secretValue := string(matches[i])
			if secretValue != "" {
				// Use the path-aware tokenization method
				token := tokenizer.TokenizeValueWithPath(secretValue, context, filePath)
				substStr = fmt.Sprintf("%s%s", substStr, token)
			} else {
				substStr = fmt.Sprintf("%s%s", substStr, MASK_TEXT)
			}
		} else if name == "drop" {
			// no-op, string is just dropped from result
		} else {
			// Named group - preserve as is
			substStr = fmt.Sprintf("%s${%s}", substStr, name)
		}
	}
	return re.ReplaceAll(line, []byte(substStr))
}

func readLine(r *bufio.Reader) ([]byte, error) {
	var completeLine []byte
	for {
		var line []byte
		line, isPrefix, err := r.ReadLine()
		if err != nil {
			return nil, err
		}

		completeLine = append(completeLine, line...)
		if !isPrefix {
			break
		}
	}
	return completeLine, nil
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

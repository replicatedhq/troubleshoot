package redact

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
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
	// tokenization controls (phase-1: local toggles; to be wired later)
	enableTokenization = false
	enableOwnerMapping = false

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
	// Enable tokenization only when explicitly requested to preserve legacy behavior/tests
	if os.Getenv("TROUBLESHOOT_TOKENIZATION") == "1" {
		enableTokenization = true
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
	// Use profile-based redaction if available
	pm := GetProfileManager()
	profile, err := pm.GetActiveProfile()
	if err != nil {
		// Fallback to legacy patterns if profile system fails
		return getLegacyRedactors(path)
	}

	// Resolve profile with inheritance
	resolvedProfile, err := pm.ResolveProfile(profile.Name)
	if err != nil {
		// Fallback to legacy patterns if profile resolution fails
		return getLegacyRedactors(path)
	}

	return buildRedactorsFromProfile(resolvedProfile, path)
}

// getLegacyRedactors returns the original hardcoded redactors for backward compatibility
func getLegacyRedactors(path string) ([]Redactor, error) {

	// (?i) makes it case insensitive
	// groups named with `?P<mask>` will be masked
	// groups named with `?P<drop>` will be removed (replaced with empty strings)
	singleLines := []struct {
		regex LineRedactor
		name  string
	}{
		// YAML/JSON key-value patterns for common secrets
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:password|pwd|pass)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `password|pwd|pass`,
			},
			name: "Redact password values in YAML/JSON",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:secret|secrets|.*[-_]?secret|.*[-_]?secrets)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `secret`,
			},
			name: "Redact secret values in YAML/JSON (including openai-secret, stripe-secret, etc.)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:api[-_]?key|apikey|.*[-_]?key|.*[-_]?api[-_]?key)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `key|api`,
			},
			name: "Redact API key values in YAML/JSON (including openai-key, stripe-key, etc.)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:token|auth[-_]?token|access[-_]?token|.*[-_]?token)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `token`,
			},
			name: "Redact token values in YAML/JSON (including github-token, slack-token, etc.)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:client[-_]?secret|client[-_]?key)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `client`,
			},
			name: "Redact client secret values in YAML/JSON",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:private[-_]?key|privatekey)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `private`,
			},
			name: "Redact private key values in YAML/JSON",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:username|user|userid|user[-_]?id)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `user`,
			},
			name: "Redact username values in YAML/JSON",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:database|db|database[-_]?name|db[-_]?name)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `database|db`,
			},
			name: "Redact database name values in YAML/JSON",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(\s*(?:email|mail|smtp[-_]?user|smtp[-_]?username)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `email|mail|smtp`,
			},
			name: "Redact email values in YAML/JSON",
		},
		// Environment variable patterns (KEY=value format)
		{
			regex: LineRedactor{
				regex: `(?i)(^.*(?:password|pwd|pass).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
				scan:  `password|pwd|pass`,
			},
			name: "Redact password environment variables (KEY=value format)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(^.*(?:secret|secrets).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
				scan:  `secret`,
			},
			name: "Redact secret environment variables (KEY=value format)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(^.*(?:key|api|token).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
				scan:  `key|api|token`,
			},
			name: "Redact key/API/token environment variables (KEY=value format)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(^.*(?:user|username|userid).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
				scan:  `user`,
			},
			name: "Redact user environment variables (KEY=value format)",
		},
		{
			regex: LineRedactor{
				regex: `(?i)(^.*(?:database|db).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
				scan:  `database|db`,
			},
			name: "Redact database environment variables (KEY=value format)",
		},
		// YAML environment variable value patterns (for env: - name/value format)
		{
			regex: LineRedactor{
				regex: `(?i)(\s*value\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
				scan:  `value`,
			},
			name: "Redact environment variable values in YAML (value: format)",
		},
		// JSON environment variable patterns (unescaped quotes)
		{
			regex: LineRedactor{
				regex: `(?i)("name":"[^"]*(?:password|secret|key|token)[^"]*","value":")(?P<mask>[^"]+)(")`,
				scan:  `password|secret|key|token`,
			},
			name: "Redact JSON environment variable values (unescaped quotes)",
		},
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
				scan:  `\b(\w*:\/\/)([^:\"\/]*){1}(:)([^@\"\/]*){1}(@)([^:\"\/]*){1}(:[\d]*)?(\/)([\w\d\S-_]+){1}\b`,
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

// buildRedactorsFromProfile creates redactors from a resolved profile
func buildRedactorsFromProfile(profile *RedactionProfile, path string) ([]Redactor, error) {
	var redactors []Redactor

	for _, pattern := range profile.Patterns {
		// Skip disabled patterns
		if !pattern.Enabled {
			continue
		}

		// Create redactor based on pattern type
		switch pattern.Type {
		case "single-line":
			lineRedactor := LineRedactor{
				regex: pattern.Regex,
				scan:  pattern.Scan,
			}
			r, err := NewSingleLineRedactor(lineRedactor, MASK_TEXT, path, pattern.Name, false)
			if err != nil {
				// Log error but continue with other patterns
				continue
			}
			redactors = append(redactors, r)

		case "multi-line":
			selectorRedactor := LineRedactor{
				regex: pattern.SelectorRegex,
				scan:  pattern.Scan,
			}
			r, err := NewMultiLineRedactor(selectorRedactor, pattern.RedactorRegex, MASK_TEXT, path, pattern.Name, false)
			if err != nil {
				// Log error but continue with other patterns
				continue
			}
			redactors = append(redactors, r)

		case "yaml":
			if pattern.FilePath == "" {
				pattern.FilePath = path // Use current path if not specified
			}
			r := NewYamlRedactor(pattern.YamlPath, pattern.FilePath, pattern.Name)
			redactors = append(redactors, r)

		case "literal":
			r := literalString([]byte(pattern.Match), path, pattern.Name)
			redactors = append(redactors, r)
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
			// Insert a fixed placeholder; tokenization post-pass will swap it with a consistent token
			substStr = fmt.Sprintf("%s%s", substStr, MASK_TEXT)
		} else if name == "drop" {
			// no-op, string is just dropped from result
		} else {
			substStr = fmt.Sprintf("%s${%s}", substStr, name)
		}
	}
	return substStr
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

// inferTypeHint extracts a coarse type from a redactor name for token prefixing.
func inferTypeHint(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "password"):
		return "PASSWORD"
	case strings.Contains(n, "token"):
		return "TOKEN"
	case strings.Contains(n, "secret"):
		return "SECRET"
	case strings.Contains(n, "user"):
		return "USER"
	case strings.Contains(n, "database"):
		return "DATABASE"
	default:
		return "SECRET"
	}
}

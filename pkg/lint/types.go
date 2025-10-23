package lint

// Core types used by the lint package

type LintResult struct {
	FilePath string
	Errors   []LintError
	Warnings []LintWarning
}

type LintError struct {
	Line    int
	Column  int
	Message string
	Field   string
}

type LintWarning struct {
	Line    int
	Column  int
	Message string
	Field   string
}

type LintOptions struct {
	FilePaths   []string
	Fix         bool
	Format      string   // "text" or "json"
	ValuesFiles []string // Path to YAML files with template values (for v1beta3)
	SetValues   []string // Template values from command line (for v1beta3)
}

// HasErrors returns true if any of the results contain errors
func HasErrors(results []LintResult) bool {
	for _, result := range results {
		if len(result.Errors) > 0 {
			return true
		}
	}
	return false
}

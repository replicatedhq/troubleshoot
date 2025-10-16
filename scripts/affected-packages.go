package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// goListPackageJSON models the subset of fields we need from `go list -json` output.
// The JSON can be quite large; we intentionally only decode what we use to keep memory reasonable.
type goListPackageJSON struct {
	ImportPath string   `json:"ImportPath"`
	Deps       []string `json:"Deps"`
}

// runCommand executes a command and returns stdout as bytes with trimmed trailing newline.
func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(out, "\n"), nil
}

// listPackageWithDeps returns the full transitive dependency set for a package import path,
// including the package itself.
func listPackageWithDeps(importPath string) (map[string]struct{}, error) {
	cmd := exec.Command("go", "list", "-json", "-deps", importPath)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -json -deps %s failed: %w", importPath, err)
	}
	deps := make(map[string]struct{})
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg goListPackageJSON
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode go list json: %w", err)
		}
		if pkg.ImportPath != "" {
			deps[pkg.ImportPath] = struct{}{}
		}
		for _, d := range pkg.Deps {
			deps[d] = struct{}{}
		}
	}
	return deps, nil
}

// changedFiles returns a slice of file paths changed between baseRef and HEAD.
func changedFiles(baseRef string) ([]string, error) {
	// Use triple-dot to include merge base with baseRef, typical for PR diffs.
	out, err := runCommand("git", "diff", "--name-only", baseRef+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

// mapFilesToPackages resolves a set of Go package import paths that directly contain the changed Go files.
func mapFilesToPackages(files []string) (map[string]struct{}, error) {
	packages := make(map[string]struct{})
	// Collect unique directories that contain changed Go files.
	dirSet := make(map[string]struct{})
	for _, f := range files {
		if strings.HasPrefix(f, "vendor/") {
			continue
		}
		if filepath.Ext(f) != ".go" {
			continue
		}
		d := filepath.Dir(f)
		if d == "." {
			d = "."
		}
		dirSet[d] = struct{}{}
	}
	if len(dirSet) == 0 {
		return packages, nil
	}

	// Convert to a stable-ordered slice of directories to avoid nondeterminism.
	var dirs []string
	for d := range dirSet {
		// Ensure relative paths are treated as packages; prepend ./ for clarity.
		if strings.HasPrefix(d, "./") || d == "." {
			dirs = append(dirs, d)
		} else {
			dirs = append(dirs, "./"+d)
		}
	}
	sort.Strings(dirs)

	// `go list` accepts directories and returns their package import paths.
	args := append([]string{"list", "-f", "{{.ImportPath}}"}, dirs...)
	out, err := runCommand("go", args...)
	if err != nil {
		return nil, fmt.Errorf("go list for files failed: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		pkg := strings.TrimSpace(scanner.Text())
		if pkg != "" {
			packages[pkg] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}

// computeAffectedPackages expands directly changed packages to include reverse dependencies across the module.
// We query all packages with test dependencies (-test) to ensure test-only imports are considered.
func computeAffectedPackages(directPkgs map[string]struct{}) (map[string]struct{}, error) {
	affected := make(map[string]struct{})
	for p := range directPkgs {
		affected[p] = struct{}{}
	}
	if len(directPkgs) == 0 {
		return affected, nil
	}

	// Enumerate all packages in the module with their deps.
	// We stream decode concatenated JSON objects produced by `go list -json`.
	cmd := exec.Command("go", "list", "-json", "-deps", "-test", "./...")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -json failed: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg goListPackageJSON
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode go list json: %w", err)
		}
		// If this package is directly changed, it's already included.
		// If it depends (directly or transitively) on any changed package, include it.
		for changed := range directPkgs {
			if pkg.ImportPath == changed {
				affected[pkg.ImportPath] = struct{}{}
				break
			}
			// Linear scan over deps is acceptable given typical package counts.
			for _, dep := range pkg.Deps {
				if dep == changed {
					affected[pkg.ImportPath] = struct{}{}
					break
				}
			}
		}
	}
	return affected, nil
}

// listTestFunctions scans a directory for Go test files and returns names of functions
// that match the pattern `func TestXxx(t *testing.T)`.
func listTestFunctions(dir string) ([]string, error) {
	var tests []string
	// Regex to capture test function names. This is a simple heuristic suitable for our codebase.
	testFuncRe := regexp.MustCompile(`^func\s+(Test[\w\d_]+)\s*\(`)

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if m := testFuncRe.FindStringSubmatch(line); m != nil {
				tests = append(tests, m[1])
			}
		}
		return scanner.Err()
	}

	if err := filepath.WalkDir(dir, walkFn); err != nil {
		return nil, err
	}
	sort.Strings(tests)
	return tests, nil
}

func main() {
	baseRef := flag.String("base", "origin/main", "Git base ref to diff against (e.g., origin/main)")
	printAllOnChanges := flag.Bool("all-on-mod-change", true, "Run all tests if go.mod or go.sum changed")
	verbose := flag.Bool("v", false, "Enable verbose diagnostics to stderr")
	mode := flag.String("mode", "packages", "Output mode: 'packages' to print import paths; 'suites' to print e2e suite names")
	changedFilesCSV := flag.String("changed-files", "", "Comma-separated paths to treat as changed (bypass git)")
	changedFilesFile := flag.String("changed-files-file", "", "File with newline-separated paths to treat as changed")
	flag.Parse()

	// Determine the set of changed files: explicit list if provided, otherwise via git diff.
	var files []string
	if *changedFilesCSV != "" || *changedFilesFile != "" {
		if *changedFilesCSV != "" {
			parts := strings.Split(*changedFilesCSV, ",")
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					files = append(files, s)
				}
			}
		}
		if *changedFilesFile != "" {
			b, err := os.ReadFile(*changedFilesFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			scanner := bufio.NewScanner(bytes.NewReader(b))
			for scanner.Scan() {
				if s := strings.TrimSpace(scanner.Text()); s != "" {
					files = append(files, s)
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
	} else {
		var err error
		files, err = changedFiles(*baseRef)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if *verbose {
		fmt.Fprintln(os.Stderr, "Changed files vs base:")
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "  (none)")
		} else {
			for _, f := range files {
				fmt.Fprintln(os.Stderr, "  ", f)
			}
		}
	}

	// Track module change and CI configuration changes to drive conservative behavior.
	moduleChanged := false
	ciChanged := false
	if *printAllOnChanges {
		for _, f := range files {
			if f == "go.mod" || f == "go.sum" {
				moduleChanged = true
			}
			if strings.HasPrefix(f, "scripts/") || strings.HasPrefix(f, ".github/workflows/") {
				ciChanged = true
			}
		}
		if (moduleChanged || ciChanged) && *mode == "packages" {
			if *verbose {
				if moduleChanged {
					fmt.Fprintln(os.Stderr, "Detected module file change (go.mod/go.sum); selecting all packages ./...")
				}
				if ciChanged {
					fmt.Fprintln(os.Stderr, "Detected CI/detector change (scripts/ or .github/workflows/); selecting all packages ./...")
				}
			}
			fmt.Println("./...")
			return
		}
	}

	directPkgs, err := mapFilesToPackages(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *verbose {
		// Stable dump of direct packages
		var dirs []string
		for p := range directPkgs {
			dirs = append(dirs, p)
		}
		sort.Strings(dirs)
		fmt.Fprintln(os.Stderr, "Directly changed packages:")
		if len(dirs) == 0 {
			fmt.Fprintln(os.Stderr, "  (none)")
		} else {
			for _, p := range dirs {
				fmt.Fprintln(os.Stderr, "  ", p)
			}
		}
	}

	switch *mode {
	case "packages":
		affected, err := computeAffectedPackages(directPkgs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if *verbose {
			var dbg []string
			for p := range affected {
				dbg = append(dbg, p)
			}
			sort.Strings(dbg)
			fmt.Fprintln(os.Stderr, "Final affected packages:")
			if len(dbg) == 0 {
				fmt.Fprintln(os.Stderr, "  (none)")
			} else {
				for _, p := range dbg {
					fmt.Fprintln(os.Stderr, "  ", p)
				}
			}
		}
		// Normalize and filter import paths:
		// - Strip test variant suffixes like "pkg [pkg.test]"
		// - Exclude e2e test packages (./test/e2e/...)
		normalized := make(map[string]struct{})
		for p := range affected {
			// Trim Go test variant decorations that appear in `go list -test`
			if idx := strings.Index(p, " ["); idx != -1 {
				p = p[:idx]
			}
			// Exclude synthetic test packages like github.com/org/repo/pkg.name.test
			if strings.HasSuffix(p, ".test") {
				continue
			}
			if strings.Contains(p, "/test/e2e/") {
				continue
			}
			if p != "" {
				normalized[p] = struct{}{}
			}
		}
		var list []string
		for p := range normalized {
			list = append(list, p)
		}
		sort.Strings(list)
		for _, p := range list {
			fmt.Println(p)
		}
	case "suites":
		// Determine impacted suites by dependency mapping and direct e2e test changes,
		// then print exact test names for those suites.
		preflightRoot := "github.com/replicatedhq/troubleshoot/cmd/preflight"
		supportRoot := "github.com/replicatedhq/troubleshoot/cmd/troubleshoot"

		preflightDeps, err := listPackageWithDeps(preflightRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		supportDeps, err := listPackageWithDeps(supportRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		preflightHit := false
		supportHit := false

		// Track whether e2e test files were directly changed per suite and collect specific test names
		changedPreflightTests := make(map[string]struct{})
		changedSupportTests := make(map[string]struct{})
		preflightE2EChangedNonGo := false
		supportE2EChangedNonGo := false
		for _, f := range files {
			if strings.HasPrefix(f, "test/e2e/preflight/") {
				if strings.HasSuffix(f, "_test.go") {
					// Extract test names from just this file
					b, err := os.ReadFile(f)
					if err == nil { // ignore read errors; they will be caught later if needed
						scanner := bufio.NewScanner(bytes.NewReader(b))
						re := regexp.MustCompile(`^func\s+(Test[\w\d_]+)\s*\(`)
						for scanner.Scan() {
							line := strings.TrimSpace(scanner.Text())
							if m := re.FindStringSubmatch(line); m != nil {
								changedPreflightTests[m[1]] = struct{}{}
							}
						}
					}
					preflightHit = true
				} else {
					// Non-go change under preflight e2e; run whole suite
					preflightE2EChangedNonGo = true
					preflightHit = true
				}
			}
			if strings.HasPrefix(f, "test/e2e/support-bundle/") {
				if strings.HasSuffix(f, "_test.go") {
					b, err := os.ReadFile(f)
					if err == nil {
						scanner := bufio.NewScanner(bytes.NewReader(b))
						re := regexp.MustCompile(`^func\s+(Test[\w\d_]+)\s*\(`)
						for scanner.Scan() {
							line := strings.TrimSpace(scanner.Text())
							if m := re.FindStringSubmatch(line); m != nil {
								changedSupportTests[m[1]] = struct{}{}
							}
						}
					}
					supportHit = true
				} else {
					supportE2EChangedNonGo = true
					supportHit = true
				}
			}
		}
		for changed := range directPkgs {
			if !preflightHit {
				if _, ok := preflightDeps[changed]; ok {
					preflightHit = true
				}
			}
			if !supportHit {
				if _, ok := supportDeps[changed]; ok {
					supportHit = true
				}
			}
			if preflightHit && supportHit {
				break
			}
		}
		if *verbose {
			fmt.Fprintln(os.Stderr, "E2E suite impact:")
			fmt.Fprintf(os.Stderr, "  preflight: %v\n", preflightHit)
			fmt.Fprintf(os.Stderr, "  support-bundle: %v\n", supportHit)
		}

		// If module files or CI/detector changed, conservatively select all tests for both suites.
		if moduleChanged || ciChanged {
			preTests, err := listTestFunctions("test/e2e/preflight")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			for _, tname := range preTests {
				fmt.Printf("preflight:%s\n", tname)
			}
			sbTests, err := listTestFunctions("test/e2e/support-bundle")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			for _, tname := range sbTests {
				fmt.Printf("support-bundle:%s\n", tname)
			}
			return
		}

		// Collect tests for impacted suites and print as `<suite>:<TestName>`
		if preflightHit || supportHit {
			if preflightHit {
				toPrint := make(map[string]struct{})
				if preflightE2EChangedNonGo || len(changedPreflightTests) == 0 {
					// Run full suite if e2e non-go assets changed or no specific test names collected
					preTests, err := listTestFunctions("test/e2e/preflight")
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(2)
					}
					for _, t := range preTests {
						toPrint[t] = struct{}{}
					}
				} else {
					for t := range changedPreflightTests {
						toPrint[t] = struct{}{}
					}
				}
				var list []string
				for t := range toPrint {
					list = append(list, t)
				}
				sort.Strings(list)
				for _, tname := range list {
					fmt.Printf("preflight:%s\n", tname)
				}
			}
			if supportHit {
				toPrint := make(map[string]struct{})
				if supportE2EChangedNonGo || len(changedSupportTests) == 0 {
					sbTests, err := listTestFunctions("test/e2e/support-bundle")
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(2)
					}
					for _, t := range sbTests {
						toPrint[t] = struct{}{}
					}
				} else {
					for t := range changedSupportTests {
						toPrint[t] = struct{}{}
					}
				}
				var list []string
				for t := range toPrint {
					list = append(list, t)
				}
				sort.Strings(list)
				for _, tname := range list {
					fmt.Printf("support-bundle:%s\n", tname)
				}
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown mode; use 'packages' or 'suites'")
		os.Exit(2)
	}
}

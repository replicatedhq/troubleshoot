package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func main() {
	baseRef := flag.String("base", "origin/main", "Git base ref to diff against (e.g., origin/main)")
	printAllOnChanges := flag.Bool("all-on-mod-change", true, "Run all tests if go.mod or go.sum changed")
	verbose := flag.Bool("v", false, "Enable verbose diagnostics to stderr")
	mode := flag.String("mode", "packages", "Output mode: 'packages' to print import paths; 'suites' to print e2e suite names")
	flag.Parse()

	files, err := changedFiles(*baseRef)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
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

	// If module files changed, be conservative.
	if *printAllOnChanges {
		for _, f := range files {
			if f == "go.mod" || f == "go.sum" {
				if *mode == "packages" {
					if *verbose {
						fmt.Fprintln(os.Stderr, "Detected module file change (go.mod/go.sum); selecting all packages ./...")
					}
					fmt.Println("./...")
					return
				}
				if *mode == "suites" {
					if *verbose {
						fmt.Fprintln(os.Stderr, "Detected module file change (go.mod/go.sum); selecting all e2e suites")
					}
					fmt.Println("preflight")
					fmt.Println("support-bundle")
					return
				}
			}
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
		var list []string
		for p := range affected {
			list = append(list, p)
		}
		sort.Strings(list)
		for _, p := range list {
			fmt.Println(p)
		}
	case "suites":
		// Determine if preflight and/or support-bundle regression suites should run
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
		if preflightHit {
			fmt.Println("preflight")
		}
		if supportHit {
			fmt.Println("support-bundle")
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown mode; use 'packages' or 'suites'")
		os.Exit(2)
	}
}

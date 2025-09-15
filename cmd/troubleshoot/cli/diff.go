package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const maxInlineDiffBytes = 256 * 1024

// DiffResult represents the result of comparing two support bundles
type DiffResult struct {
	Summary      DiffSummary  `json:"summary"`
	Changes      []Change     `json:"changes"`
	Metadata     DiffMetadata `json:"metadata"`
	Significance string       `json:"significance"`
}

// DiffSummary provides high-level statistics about the diff
type DiffSummary struct {
	TotalChanges      int `json:"totalChanges"`
	FilesAdded        int `json:"filesAdded"`
	FilesRemoved      int `json:"filesRemoved"`
	FilesModified     int `json:"filesModified"`
	HighImpactChanges int `json:"highImpactChanges"`
}

// Change represents a single difference between bundles
type Change struct {
	Type        string                 `json:"type"`     // added, removed, modified
	Category    string                 `json:"category"` // resource, log, config, etc.
	Path        string                 `json:"path"`     // file path or resource path
	Impact      string                 `json:"impact"`   // high, medium, low, none
	Details     map[string]interface{} `json:"details"`  // change-specific details
	Remediation *RemediationStep       `json:"remediation,omitempty"`
}

// RemediationStep represents a suggested remediation action
type RemediationStep struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	URL         string `json:"url,omitempty"`
}

// DiffMetadata contains metadata about the diff operation
type DiffMetadata struct {
	OldBundle   BundleMetadata `json:"oldBundle"`
	NewBundle   BundleMetadata `json:"newBundle"`
	GeneratedAt string         `json:"generatedAt"`
	Version     string         `json:"version"`
}

// BundleMetadata contains metadata about a support bundle
type BundleMetadata struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"createdAt,omitempty"`
	NumFiles  int    `json:"numFiles"`
}

func Diff() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <old-bundle.(tar.gz|tgz)> <new-bundle.(tar.gz|tgz)>",
		Args:  cobra.ExactArgs(2),
		Short: "Compare two support bundles and identify changes",
		Long: `Compare two support bundles to identify changes over time.
This command analyzes differences between two support bundle archives and generates
a human-readable report showing what has changed, including:

- Added, removed, or modified files
- Configuration changes
- Log differences
- Resource status changes
- Performance metric changes

Use -o to write the report to a file; otherwise it prints to stdout.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			return runBundleDiff(v, args[0], args[1])
		},
	}

	cmd.Flags().StringP("output", "o", "", "file path of where to save the diff report (default prints to stdout)")
	cmd.Flags().Int("max-diff-lines", 200, "maximum total lines to include in an inline diff for a single file")
	cmd.Flags().Int("max-diff-files", 50, "maximum number of files to include inline diffs for; additional modified files will omit inline diffs")
	cmd.Flags().Bool("include-log-diffs", false, "include inline diffs for log files as well")
	cmd.Flags().Int("diff-context", 3, "number of context lines to include around changes in unified diffs")
	cmd.Flags().Bool("hide-inline-diffs", false, "hide inline unified diffs in the report")
	cmd.Flags().String("format", "", "output format; set to 'json' to emit machine-readable JSON to stdout or -o")

	return cmd
}

func runBundleDiff(v *viper.Viper, oldBundle, newBundle string) error {
	klog.V(2).Infof("Comparing support bundles: %s -> %s", oldBundle, newBundle)

	// Validate input files
	if err := validateBundleFile(oldBundle); err != nil {
		return errors.Wrap(err, "invalid old bundle")
	}
	if err := validateBundleFile(newBundle); err != nil {
		return errors.Wrap(err, "invalid new bundle")
	}

	// Perform the diff (placeholder implementation)
	diffResult, err := performBundleDiff(oldBundle, newBundle, v)
	if err != nil {
		return errors.Wrap(err, "failed to compare bundles")
	}

	// Output the results
	if err := outputDiffResult(diffResult, v); err != nil {
		return errors.Wrap(err, "failed to output diff results")
	}

	return nil
}

func validateBundleFile(bundlePath string) error {
	if bundlePath == "" {
		return errors.New("bundle path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle file not found: %s", bundlePath)
	}

	// Support .tar.gz and .tgz bundles
	lower := strings.ToLower(bundlePath)
	if !(strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")) {
		return fmt.Errorf("unsupported bundle format. Expected path to end with .tar.gz or .tgz")
	}

	return nil
}

func performBundleDiff(oldBundle, newBundle string, v *viper.Viper) (*DiffResult, error) {
	klog.V(2).Info("Performing bundle diff analysis (streaming)...")

	// Stream inventories
	oldInv, err := buildInventoryFromTarGz(oldBundle)
	if err != nil {
		return nil, errors.Wrap(err, "failed to inventory old bundle")
	}
	newInv, err := buildInventoryFromTarGz(newBundle)
	if err != nil {
		return nil, errors.Wrap(err, "failed to inventory new bundle")
	}

	var changes []Change
	inlineDiffsIncluded := 0
	maxDiffLines := v.GetInt("max-diff-lines")
	if maxDiffLines <= 0 {
		maxDiffLines = 200
	}
	maxDiffFiles := v.GetInt("max-diff-files")
	if maxDiffFiles <= 0 {
		maxDiffFiles = 50
	}
	includeLogDiffs := v.GetBool("include-log-diffs")
	diffContext := v.GetInt("diff-context")
	if diffContext <= 0 {
		diffContext = 3
	}

	// Added files
	for p, nf := range newInv {
		if _, ok := oldInv[p]; !ok {
			ch := Change{
				Type:     "added",
				Category: categorizePath(p),
				Path:     "/" + p,
				Impact:   estimateImpact("added", p),
				Details: map[string]interface{}{
					"size": nf.Size,
				},
			}
			if rem := suggestRemediation(ch.Type, p); rem != nil {
				ch.Remediation = rem
			}
			changes = append(changes, ch)
		}
	}

	// Removed files
	for p, of := range oldInv {
		if _, ok := newInv[p]; !ok {
			ch := Change{
				Type:     "removed",
				Category: categorizePath(p),
				Path:     "/" + p,
				Impact:   estimateImpact("removed", p),
				Details: map[string]interface{}{
					"size": of.Size,
				},
			}
			if rem := suggestRemediation(ch.Type, p); rem != nil {
				ch.Remediation = rem
			}
			changes = append(changes, ch)
		}
	}

	// Modified files
	for p, of := range oldInv {
		if nf, ok := newInv[p]; ok {
			if of.Digest != nf.Digest {
				ch := Change{
					Type:     "modified",
					Category: categorizePath(p),
					Path:     "/" + p,
					Impact:   estimateImpact("modified", p),
					Details:  map[string]interface{}{},
				}
				if rem := suggestRemediation(ch.Type, p); rem != nil {
					ch.Remediation = rem
				}
				changes = append(changes, ch)
			}
		}
	}

	// Sort changes deterministically: type, then path
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type == changes[j].Type {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Type < changes[j].Type
	})

	// Populate inline diffs lazily for the first N eligible modified files using streaming approach
	for i := range changes {
		if inlineDiffsIncluded >= maxDiffFiles {
			break
		}
		ch := &changes[i]
		if ch.Type != "modified" {
			continue
		}
		allowLogs := includeLogDiffs || ch.Category != "logs"
		if !allowLogs {
			continue
		}
		// Use streaming diff generation to avoid loading large files into memory
		if d := generateStreamingUnifiedDiff(oldBundle, newBundle, ch.Path, diffContext, maxDiffLines); d != "" {
			if ch.Details == nil {
				ch.Details = map[string]interface{}{}
			}
			ch.Details["diff"] = d
			inlineDiffsIncluded++
		}
	}

	// Summaries
	summary := DiffSummary{}
	for _, c := range changes {
		switch c.Type {
		case "added":
			summary.FilesAdded++
		case "removed":
			summary.FilesRemoved++
		case "modified":
			summary.FilesModified++
		}
		if c.Impact == "high" {
			summary.HighImpactChanges++
		}
	}
	summary.TotalChanges = summary.FilesAdded + summary.FilesRemoved + summary.FilesModified

	oldMeta := getBundleMetadataWithCount(oldBundle, len(oldInv))
	newMeta := getBundleMetadataWithCount(newBundle, len(newInv))

	result := &DiffResult{
		Summary:      summary,
		Changes:      changes,
		Metadata:     DiffMetadata{OldBundle: oldMeta, NewBundle: newMeta, GeneratedAt: time.Now().Format(time.RFC3339), Version: "v1"},
		Significance: computeOverallSignificance(changes),
	}
	return result, nil
}

type inventoryFile struct {
	Size   int64
	Digest string
}

func buildInventoryFromTarGz(bundlePath string) (map[string]inventoryFile, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open bundle")
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gzip reader")
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	inv := make(map[string]inventoryFile)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read tar entry")
		}
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}

		norm := normalizePath(hdr.Name)
		if norm == "" {
			continue
		}

		h := sha256.New()
		var copied int64
		buf := make([]byte, 32*1024)
		for copied < hdr.Size {
			toRead := int64(len(buf))
			if remain := hdr.Size - copied; remain < toRead {
				toRead = remain
			}
			n, rerr := io.ReadFull(tr, buf[:toRead])
			if n > 0 {
				_, _ = h.Write(buf[:n])
				copied += int64(n)
			}
			if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
				break
			}
			if rerr != nil {
				return nil, errors.Wrap(rerr, "failed to read file content")
			}
		}

		digest := hex.EncodeToString(h.Sum(nil))
		inv[norm] = inventoryFile{Size: hdr.Size, Digest: digest}
	}

	return inv, nil
}

func normalizePath(name string) string {
	name = strings.TrimPrefix(name, "./")
	if name == "" {
		return name
	}
	i := strings.IndexByte(name, '/')
	if i < 0 {
		return name
	}
	first := name[:i]
	rest := name[i+1:]

	// Known domain roots we do not strip
	domainRoots := map[string]bool{
		"cluster-resources": true,
		"all-logs":          true,
		"cluster-info":      true,
		"execution-data":    true,
	}
	if domainRoots[first] {
		return name
	}
	// Strip only known container prefixes
	if first == "root" || strings.HasPrefix(strings.ToLower(first), "support-bundle") {
		return rest
	}
	return name
}

func isProbablyText(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}
	for _, b := range sample {
		if b == 0x00 {
			return false
		}
		if b < 0x09 {
			return false
		}
	}
	return true
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// generateStreamingUnifiedDiff creates a unified diff by streaming files line-by-line to avoid loading large files into memory
func generateStreamingUnifiedDiff(oldBundle, newBundle, path string, context, maxTotalLines int) string {
	oldReader, err := createTarFileReader(oldBundle, strings.TrimPrefix(path, "/"))
	if err != nil {
		return ""
	}
	defer oldReader.Close()

	newReader, err := createTarFileReader(newBundle, strings.TrimPrefix(path, "/"))
	if err != nil {
		return ""
	}
	defer newReader.Close()

	// Read files line by line
	oldLines, err := readLinesFromReader(oldReader, maxInlineDiffBytes)
	if err != nil {
		return ""
	}

	newLines, err := readLinesFromReader(newReader, maxInlineDiffBytes)
	if err != nil {
		return ""
	}

	// Generate diff using the existing difflib
	ud := difflib.UnifiedDiff{
		A:        oldLines,
		B:        newLines,
		FromFile: "old:" + path,
		ToFile:   "new:" + path,
		Context:  context,
	}
	s, err := difflib.GetUnifiedDiffString(ud)
	if err != nil || s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	if maxTotalLines > 0 && len(lines) > maxTotalLines {
		lines = append(lines[:maxTotalLines], "... (diff truncated)")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// readLinesFromReader reads lines from a reader up to maxBytes total, returning normalized lines
func readLinesFromReader(reader io.Reader, maxBytes int) ([]string, error) {
	var lines []string
	var totalBytes int
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := normalizeNewlines(scanner.Text())
		lineBytes := len(line) + 1 // +1 for newline

		if totalBytes+lineBytes > maxBytes {
			lines = append(lines, "... (content truncated due to size)")
			break
		}

		lines = append(lines, line)
		totalBytes += lineBytes
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// generateUnifiedDiff builds a unified diff with headers and context, then truncates to maxTotalLines
func generateUnifiedDiff(a, b string, path string, context, maxTotalLines int) string {
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(a),
		B:        difflib.SplitLines(b),
		FromFile: "old:" + path,
		ToFile:   "new:" + path,
		Context:  context,
	}
	s, err := difflib.GetUnifiedDiffString(ud)
	if err != nil || s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if maxTotalLines > 0 && len(lines) > maxTotalLines {
		lines = append(lines[:maxTotalLines], "... (diff truncated)")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func categorizePath(p string) string {
	if strings.HasPrefix(p, "cluster-resources/pods/logs/") || strings.Contains(p, "/logs/") || strings.HasPrefix(p, "all-logs/") || strings.HasPrefix(p, "logs/") {
		return "logs"
	}
	if strings.HasPrefix(p, "cluster-resources/") {
		rest := strings.TrimPrefix(p, "cluster-resources/")
		seg := rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			seg = rest[:i]
		}
		if seg == "" {
			return "resources"
		}
		return "resources:" + seg
	}
	if strings.HasPrefix(p, "config/") || strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml") || strings.HasSuffix(p, ".json") {
		return "config"
	}
	return "files"
}

// estimateImpact determines impact based on change type and path patterns
func estimateImpact(changeType, p string) string {
	// High impact cases
	if strings.HasPrefix(p, "cluster-resources/custom-resource-definitions") {
		if changeType == "removed" || changeType == "modified" {
			return "high"
		}
	}
	if strings.HasPrefix(p, "cluster-resources/clusterrole") || strings.HasPrefix(p, "cluster-resources/clusterrolebindings") || strings.Contains(p, "/rolebindings/") {
		if changeType != "added" { // reductions or changes can break access
			return "high"
		}
	}
	if strings.Contains(p, "/secrets/") || strings.HasSuffix(p, "-secrets.json") {
		if changeType != "added" {
			return "high"
		}
	}
	if strings.HasPrefix(p, "cluster-resources/nodes") {
		if changeType != "added" { // node status changes can be severe
			return "high"
		}
	}
	if strings.Contains(p, "/network-policy/") || strings.HasSuffix(p, "/networkpolicies.json") {
		if changeType != "added" {
			return "high"
		}
	}
	if strings.HasPrefix(p, "cluster-resources/") && strings.Contains(p, "/kube-system") {
		if changeType != "added" {
			return "high"
		}
	}
	// Medium default for cluster resources
	if strings.HasPrefix(p, "cluster-resources/") {
		return "medium"
	}
	// Logs and others default low
	if strings.Contains(p, "/logs/") || strings.HasPrefix(p, "all-logs/") {
		return "low"
	}
	return "low"
}

// suggestRemediation returns a basic remediation suggestion based on category and change
func suggestRemediation(changeType, p string) *RemediationStep {
	// RBAC
	if strings.HasPrefix(p, "cluster-resources/clusterrole") || strings.HasPrefix(p, "cluster-resources/clusterrolebindings") || strings.Contains(p, "/rolebindings/") {
		return &RemediationStep{Description: "Validate RBAC permissions and recent changes", Command: "kubectl auth can-i --list"}
	}
	// CRDs
	if strings.HasPrefix(p, "cluster-resources/custom-resource-definitions") {
		return &RemediationStep{Description: "Check CRD presence and reconcile operator status", Command: "kubectl get crds"}
	}
	// Nodes
	if strings.HasPrefix(p, "cluster-resources/nodes") {
		return &RemediationStep{Description: "Inspect node conditions and recent events", Command: "kubectl describe nodes"}
	}
	// Network policy
	if strings.Contains(p, "/network-policy/") || strings.HasSuffix(p, "/networkpolicies.json") {
		return &RemediationStep{Description: "Validate connectivity and recent NetworkPolicy changes", Command: "kubectl get networkpolicy -A"}
	}
	// Secrets/Config
	if strings.Contains(p, "/secrets/") || strings.HasPrefix(p, "config/") {
		return &RemediationStep{Description: "Review configuration or secret changes for correctness"}
	}
	// Workloads
	if strings.Contains(p, "/deployments/") || strings.Contains(p, "/statefulsets/") || strings.Contains(p, "/daemonsets/") {
		return &RemediationStep{Description: "Check rollout and pod status", Command: "kubectl rollout status -n <ns> <kind>/<name>"}
	}
	return nil
}

func computeOverallSignificance(changes []Change) string {
	high, medium := 0, 0
	for _, c := range changes {
		switch c.Impact {
		case "high":
			high++
		case "medium":
			medium++
		}
	}
	if high > 0 {
		return "high"
	}
	if medium > 0 {
		return "medium"
	}
	if len(changes) > 0 {
		return "low"
	}
	return "none"
}

func getBundleMetadata(bundlePath string) BundleMetadata {
	metadata := BundleMetadata{
		Path: bundlePath,
	}

	if stat, err := os.Stat(bundlePath); err == nil {
		metadata.Size = stat.Size()
		metadata.CreatedAt = stat.ModTime().Format(time.RFC3339)
	}

	return metadata
}

// getBundleMetadataWithCount sets NumFiles directly to avoid re-reading the archive
func getBundleMetadataWithCount(bundlePath string, numFiles int) BundleMetadata {
	md := getBundleMetadata(bundlePath)
	md.NumFiles = numFiles
	return md
}

func outputDiffResult(result *DiffResult, v *viper.Viper) error {
	outputPath := v.GetString("output")
	showInlineDiffs := !v.GetBool("hide-inline-diffs")
	formatMode := strings.ToLower(v.GetString("format"))

	var output []byte
	if formatMode == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal diff result to JSON")
		}
		output = data
	} else {
		output = []byte(generateTextDiffReport(result, showInlineDiffs))
	}

	if outputPath != "" {
		// Write to file
		if err := os.WriteFile(outputPath, output, 0644); err != nil {
			return errors.Wrap(err, "failed to write diff output to file")
		}
		fmt.Printf("Diff report written to: %s\n", outputPath)
	} else {
		// Write to stdout
		fmt.Print(string(output))
	}

	return nil
}

func generateTextDiffReport(result *DiffResult, showInlineDiffs bool) string {
	var report strings.Builder

	report.WriteString("Support Bundle Diff Report\n")
	report.WriteString("==========================\n\n")

	report.WriteString(fmt.Sprintf("Generated: %s\n", result.Metadata.GeneratedAt))
	report.WriteString(fmt.Sprintf("Old Bundle: %s (%s)\n", result.Metadata.OldBundle.Path, formatSize(result.Metadata.OldBundle.Size)))
	report.WriteString(fmt.Sprintf("New Bundle: %s (%s)\n\n", result.Metadata.NewBundle.Path, formatSize(result.Metadata.NewBundle.Size)))

	// Summary
	report.WriteString("Summary:\n")
	report.WriteString(fmt.Sprintf("  Total Changes: %d\n", result.Summary.TotalChanges))
	report.WriteString(fmt.Sprintf("  Files Added: %d\n", result.Summary.FilesAdded))
	report.WriteString(fmt.Sprintf("  Files Removed: %d\n", result.Summary.FilesRemoved))
	report.WriteString(fmt.Sprintf("  Files Modified: %d\n", result.Summary.FilesModified))
	report.WriteString(fmt.Sprintf("  High Impact Changes: %d\n", result.Summary.HighImpactChanges))
	report.WriteString(fmt.Sprintf("  Significance: %s\n\n", result.Significance))

	if len(result.Changes) == 0 {
		report.WriteString("No changes detected between bundles.\n")
	} else {
		report.WriteString("Changes:\n")
		for i, change := range result.Changes {
			report.WriteString(fmt.Sprintf("  %d. [%s] %s (%s impact)\n",
				i+1, strings.ToUpper(change.Type), change.Path, change.Impact))
			if change.Remediation != nil {
				report.WriteString(fmt.Sprintf("     Remediation: %s\n", change.Remediation.Description))
			}
			if showInlineDiffs {
				if diffStr, ok := change.Details["diff"].(string); ok && diffStr != "" {
					report.WriteString("     Diff:\n")
					for _, line := range strings.Split(diffStr, "\n") {
						report.WriteString("       " + line + "\n")
					}
				}
			}
		}
	}

	return report.String()
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// tarFileReader provides a streaming interface to read a specific file from a tar.gz archive
type tarFileReader struct {
	file   *os.File
	gz     *gzip.Reader
	tr     *tar.Reader
	found  bool
	header *tar.Header
}

// createTarFileReader creates a streaming reader for a specific file in a tar.gz archive
func createTarFileReader(bundlePath, normalizedPath string) (*tarFileReader, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	tr := tar.NewReader(gz)

	// Find the target file
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			gz.Close()
			f.Close()
			return nil, err
		}
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}
		if normalizePath(hdr.Name) == normalizedPath {
			// Check if file is probably text
			sample := make([]byte, 512)
			n, _ := io.ReadFull(tr, sample[:])
			if n > 0 && !isProbablyText(sample[:n]) {
				gz.Close()
				f.Close()
				return nil, fmt.Errorf("file is not text")
			}

			// Reopen to start from beginning of file
			gz.Close()
			f.Close()

			f, err = os.Open(bundlePath)
			if err != nil {
				return nil, err
			}
			gz, err = gzip.NewReader(f)
			if err != nil {
				f.Close()
				return nil, err
			}
			tr = tar.NewReader(gz)

			// Find the file again
			for {
				hdr, err = tr.Next()
				if err == io.EOF {
					gz.Close()
					f.Close()
					return nil, fmt.Errorf("file not found on second pass")
				}
				if err != nil {
					gz.Close()
					f.Close()
					return nil, err
				}
				if normalizePath(hdr.Name) == normalizedPath {
					return &tarFileReader{
						file:   f,
						gz:     gz,
						tr:     tr,
						found:  true,
						header: hdr,
					}, nil
				}
			}
		}
	}

	gz.Close()
	f.Close()
	return nil, fmt.Errorf("file not found: %s", normalizedPath)
}

// Read implements io.Reader interface
func (r *tarFileReader) Read(p []byte) (n int, err error) {
	if !r.found {
		return 0, io.EOF
	}
	return r.tr.Read(p)
}

// Close closes the underlying file handles
func (r *tarFileReader) Close() error {
	if r.gz != nil {
		r.gz.Close()
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

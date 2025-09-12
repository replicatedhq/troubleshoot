package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

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
		Use:   "diff <old-bundle.tgz> <new-bundle.tgz>",
		Args:  cobra.ExactArgs(2),
		Short: "Compare two support bundles and identify changes",
		Long: `Compare two support bundles to identify changes over time.
This command analyzes differences between two support bundle archives and generates
a detailed report showing what has changed, including:

- Added, removed, or modified files
- Configuration changes
- Log differences
- Resource status changes
- Performance metric changes

The output can be formatted as JSON for programmatic consumption or as
human-readable text for manual review.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			return runBundleDiff(v, args[0], args[1])
		},
	}

	cmd.Flags().String("output", "text", "output format: text, json, html")
	cmd.Flags().StringP("output-file", "f", "", "write diff output to file instead of stdout")
	cmd.Flags().Bool("ignore-timestamps", true, "ignore timestamp differences in logs and events")
	cmd.Flags().Bool("ignore-order", true, "ignore ordering differences in arrays and lists")
	cmd.Flags().StringSlice("ignore-paths", []string{}, "file paths to ignore in diff (supports glob patterns)")
	cmd.Flags().String("significance-threshold", "medium", "minimum significance level to report: low, medium, high")
	cmd.Flags().Bool("include-remediation", true, "include remediation suggestions in diff output")
	cmd.Flags().Bool("verbose", false, "include detailed diff information")

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

	// Check if it's a valid archive format
	ext := strings.ToLower(filepath.Ext(bundlePath))
	validExtensions := []string{".tgz", ".tar.gz", ".zip"}

	isValid := false
	for _, validExt := range validExtensions {
		if strings.HasSuffix(bundlePath, validExt) {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("unsupported bundle format. Expected: %v, got: %s", validExtensions, ext)
	}

	return nil
}

func performBundleDiff(oldBundle, newBundle string, v *viper.Viper) (*DiffResult, error) {
	// This is a placeholder implementation
	// In the full implementation, this would:
	// 1. Extract both bundles to temporary directories
	// 2. Compare files and contents
	// 3. Generate change analysis
	// 4. Create remediation suggestions

	klog.V(2).Info("Performing bundle diff analysis...")

	// Get bundle metadata
	oldMeta := getBundleMetadata(oldBundle)
	newMeta := getBundleMetadata(newBundle)

	// Create placeholder diff result
	result := &DiffResult{
		Summary: DiffSummary{
			TotalChanges:      0,
			FilesAdded:        0,
			FilesRemoved:      0,
			FilesModified:     0,
			HighImpactChanges: 0,
		},
		Changes: []Change{},
		Metadata: DiffMetadata{
			OldBundle:   oldMeta,
			NewBundle:   newMeta,
			GeneratedAt: fmt.Sprintf("%v", time.Now().Format(time.RFC3339)),
			Version:     "v1",
		},
		Significance: "none",
	}

	// TODO: Implement actual diff logic in Phase 4 (Support Bundle Differencing)
	klog.V(1).Info("Note: Full diff implementation will be completed in Phase 4")

	return result, nil
}

func getBundleMetadata(bundlePath string) BundleMetadata {
	metadata := BundleMetadata{
		Path: bundlePath,
	}

	if stat, err := os.Stat(bundlePath); err == nil {
		metadata.Size = stat.Size()
		metadata.CreatedAt = stat.ModTime().Format(time.RFC3339)
	}

	// Get actual file count from bundle
	fileCount, err := getFileCountFromBundle(bundlePath)
	if err != nil {
		klog.V(2).Infof("Failed to get file count for bundle %s: %v", bundlePath, err)
		metadata.NumFiles = 0
	} else {
		metadata.NumFiles = fileCount
	}

	return metadata
}

// getFileCountFromBundle counts the number of files in a support bundle archive
func getFileCountFromBundle(bundlePath string) (int, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open bundle file")
	}
	defer file.Close()

	// Check if it's a gzipped tar file
	var reader io.Reader = file
	if strings.HasSuffix(bundlePath, ".gz") || strings.HasSuffix(bundlePath, ".tgz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, errors.Wrap(err, "failed to create gzip reader")
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	tarReader := tar.NewReader(reader)
	fileCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.Wrap(err, "failed to read tar entry")
		}

		// Count regular files (not directories)
		if header.Typeflag == tar.TypeReg {
			fileCount++
		}
	}

	return fileCount, nil
}

func outputDiffResult(result *DiffResult, v *viper.Viper) error {
	outputFormat := v.GetString("output")
	outputFile := v.GetString("output-file")

	var output []byte
	var err error

	switch outputFormat {
	case "json":
		output, err = json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal diff result to JSON")
		}
	case "html":
		output = []byte(generateHTMLDiffReport(result))
	case "text":
		fallthrough
	default:
		output = []byte(generateTextDiffReport(result))
	}

	if outputFile != "" {
		// Write to file
		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return errors.Wrap(err, "failed to write diff output to file")
		}
		fmt.Printf("Diff report written to: %s\n", outputFile)
	} else {
		// Write to stdout
		fmt.Print(string(output))
	}

	return nil
}

func generateTextDiffReport(result *DiffResult) string {
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
		}
	}

	return report.String()
}

func generateHTMLDiffReport(result *DiffResult) string {
	// Basic HTML report template
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Support Bundle Diff Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .summary { background: #f5f5f5; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .change { margin: 10px 0; padding: 10px; border-left: 4px solid #ccc; }
        .change.added { border-color: #28a745; }
        .change.removed { border-color: #dc3545; }
        .change.modified { border-color: #ffc107; }
        .impact-high { background: #f8d7da; }
        .impact-medium { background: #fff3cd; }
        .impact-low { background: #d1ecf1; }
    </style>
</head>
<body>
    <h1>Support Bundle Diff Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p><strong>Generated:</strong> %s</p>
        <p><strong>Old Bundle:</strong> %s</p>
        <p><strong>New Bundle:</strong> %s</p>
        <p><strong>Total Changes:</strong> %d</p>
        <p><strong>Significance:</strong> %s</p>
    </div>
    <h2>Changes</h2>
    %s
</body>
</html>`,
		result.Metadata.GeneratedAt,
		result.Metadata.OldBundle.Path,
		result.Metadata.NewBundle.Path,
		result.Summary.TotalChanges,
		result.Significance,
		generateHTMLChanges(result.Changes))

	return html
}

func generateHTMLChanges(changes []Change) string {
	if len(changes) == 0 {
		return "<p>No changes detected between bundles.</p>"
	}

	var html strings.Builder
	for _, change := range changes {
		impactClass := fmt.Sprintf("impact-%s", change.Impact)
		changeClass := change.Type

		html.WriteString(fmt.Sprintf(`<div class="change %s %s">`, changeClass, impactClass))
		html.WriteString(fmt.Sprintf(`<strong>[%s]</strong> %s <em>(%s impact)</em>`,
			strings.ToUpper(change.Type), change.Path, change.Impact))

		if change.Remediation != nil {
			html.WriteString(fmt.Sprintf(`<br><strong>Remediation:</strong> %s`, change.Remediation.Description))
		}

		html.WriteString("</div>")
	}

	return html.String()
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

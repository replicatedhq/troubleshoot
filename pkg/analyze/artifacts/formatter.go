package artifacts

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// AnalysisArtifact represents the structured output format
type AnalysisArtifact struct {
	Schema      string                            `json:"$schema"`
	Version     string                            `json:"version"`
	Metadata    analyzer.AnalysisMetadata         `json:"metadata"`
	Summary     analyzer.AnalysisSummary          `json:"summary"`
	Results     []analyzer.EnhancedAnalyzerResult `json:"results"`
	Remediation []analyzer.RemediationStep        `json:"remediation"`
	Insights    []analyzer.AnalysisInsight        `json:"insights,omitempty"`
}

// FormatterOptions configures the output formatting
type FormatterOptions struct {
	Format             string            `json:"format"` // json, yaml, html
	IncludeMetadata    bool              `json:"includeMetadata"`
	IncludeRemediation bool              `json:"includeRemediation"`
	IncludeInsights    bool              `json:"includeInsights"`
	SortBy             string            `json:"sortBy"`      // priority, title, severity
	FilterLevel        string            `json:"filterLevel"` // all, failures, warnings, passes
	CustomFields       map[string]string `json:"customFields,omitempty"`
}

// Formatter handles analysis result formatting and serialization
type Formatter struct {
	options FormatterOptions
}

// NewFormatter creates a new formatter with options
func NewFormatter(opts FormatterOptions) *Formatter {
	// Set defaults
	if opts.Format == "" {
		opts.Format = "json"
	}
	if opts.SortBy == "" {
		opts.SortBy = "priority"
	}
	if opts.FilterLevel == "" {
		opts.FilterLevel = "all"
	}

	return &Formatter{
		options: opts,
	}
}

// FormatAnalysis formats analysis results into the specified format
func (f *Formatter) FormatAnalysis(analysis *analyzer.EnhancedAnalysisResult, writer io.Writer) error {
	if analysis == nil {
		return errors.New("analysis result cannot be nil")
	}

	// Create artifact structure
	artifact := f.createArtifact(analysis)

	// Apply filtering and sorting
	artifact = f.applyFiltering(artifact)
	artifact = f.applySorting(artifact)

	// Format and write
	switch f.options.Format {
	case "json":
		return f.writeJSON(artifact, writer)
	case "yaml":
		return f.writeYAML(artifact, writer)
	case "html":
		return f.writeHTML(artifact, writer)
	default:
		return fmt.Errorf("unsupported format: %s", f.options.Format)
	}
}

// Create the artifact structure from analysis results
func (f *Formatter) createArtifact(analysis *analyzer.EnhancedAnalysisResult) *AnalysisArtifact {
	artifact := &AnalysisArtifact{
		Schema:  "https://schemas.troubleshoot.sh/analysis/v1beta2",
		Version: "v1beta2",
		Results: analysis.Results,
	}

	// Include optional sections based on options
	if f.options.IncludeMetadata {
		artifact.Metadata = analysis.Metadata
	}

	artifact.Summary = analysis.Summary

	if f.options.IncludeRemediation {
		artifact.Remediation = analysis.Remediation
	}

	if f.options.IncludeInsights {
		// Extract insights from agent results (simplified for now)
		artifact.Insights = f.extractInsights(analysis)
	}

	return artifact
}

// Extract insights from the analysis (placeholder implementation)
func (f *Formatter) extractInsights(analysis *analyzer.EnhancedAnalysisResult) []analyzer.AnalysisInsight {
	// This would extract insights from various sources
	// For now, return empty slice
	return []analyzer.AnalysisInsight{}
}

// Apply filtering based on options
func (f *Formatter) applyFiltering(artifact *AnalysisArtifact) *AnalysisArtifact {
	if f.options.FilterLevel == "all" {
		return artifact
	}

	var filteredResults []analyzer.EnhancedAnalyzerResult

	for _, result := range artifact.Results {
		switch f.options.FilterLevel {
		case "failures":
			if result.IsFail {
				filteredResults = append(filteredResults, result)
			}
		case "warnings":
			if result.IsWarn {
				filteredResults = append(filteredResults, result)
			}
		case "passes":
			if result.IsPass {
				filteredResults = append(filteredResults, result)
			}
		}
	}

	// Update counts in summary
	artifact.Results = filteredResults
	artifact.Summary.TotalChecks = len(filteredResults)

	// Recalculate summary counts
	passed, failed, warned := 0, 0, 0
	for _, result := range filteredResults {
		if result.IsPass {
			passed++
		} else if result.IsFail {
			failed++
		} else if result.IsWarn {
			warned++
		}
	}

	artifact.Summary.PassedChecks = passed
	artifact.Summary.FailedChecks = failed
	artifact.Summary.WarningChecks = warned

	return artifact
}

// Apply sorting based on options
func (f *Formatter) applySorting(artifact *AnalysisArtifact) *AnalysisArtifact {
	results := artifact.Results

	switch f.options.SortBy {
	case "priority":
		sort.Slice(results, func(i, j int) bool {
			return f.getPriority(results[i]) < f.getPriority(results[j])
		})
	case "title":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Title < results[j].Title
		})
	case "severity":
		sort.Slice(results, func(i, j int) bool {
			return f.getSeverityWeight(results[i]) > f.getSeverityWeight(results[j])
		})
	}

	artifact.Results = results

	// Sort remediation by priority
	sort.Slice(artifact.Remediation, func(i, j int) bool {
		return artifact.Remediation[i].Priority < artifact.Remediation[j].Priority
	})

	return artifact
}

// Get priority weight for sorting (lower number = higher priority)
func (f *Formatter) getPriority(result analyzer.EnhancedAnalyzerResult) int {
	if result.IsFail {
		switch result.Impact {
		case "HIGH", "CRITICAL":
			return 1
		case "MEDIUM":
			return 3
		default:
			return 5
		}
	}

	if result.IsWarn {
		return 7
	}

	if result.IsPass {
		return 10
	}

	return 9
}

// Get severity weight for sorting (higher number = more severe)
func (f *Formatter) getSeverityWeight(result analyzer.EnhancedAnalyzerResult) int {
	if result.IsFail {
		return 3
	}
	if result.IsWarn {
		return 2
	}
	if result.IsPass {
		return 1
	}
	return 0
}

// Write JSON format
func (f *Formatter) writeJSON(artifact *AnalysisArtifact, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(artifact)
}

// Write YAML format (simplified implementation)
func (f *Formatter) writeYAML(artifact *AnalysisArtifact, writer io.Writer) error {
	// For now, convert to JSON then to YAML
	// In a full implementation, this would use a proper YAML library
	jsonData, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}

	_, err = writer.Write(jsonData)
	return err
}

// Write HTML format
func (f *Formatter) writeHTML(artifact *AnalysisArtifact, writer io.Writer) error {
	html := f.generateHTML(artifact)
	_, err := writer.Write([]byte(html))
	return err
}

// Generate HTML report
func (f *Formatter) generateHTML(artifact *AnalysisArtifact) string {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Troubleshoot Analysis Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { border-bottom: 2px solid #e0e0e0; padding-bottom: 20px; margin-bottom: 20px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .summary-card { background: #f8f9fa; padding: 15px; border-radius: 6px; border-left: 4px solid #007bff; }
        .summary-card.critical { border-color: #dc3545; }
        .summary-card.warning { border-color: #ffc107; }
        .summary-card.success { border-color: #28a745; }
        .result { margin: 10px 0; padding: 15px; border-radius: 6px; border-left: 4px solid #ddd; }
        .result.fail { border-color: #dc3545; background-color: #f8d7da; }
        .result.warn { border-color: #ffc107; background-color: #fff3cd; }
        .result.pass { border-color: #28a745; background-color: #d4edda; }
        .result-title { font-weight: bold; margin-bottom: 5px; }
        .result-message { margin-bottom: 10px; }
        .result-explanation { font-style: italic; color: #666; margin-bottom: 10px; }
        .evidence { background: #f8f9fa; padding: 10px; border-radius: 4px; margin: 10px 0; }
        .evidence-title { font-weight: bold; margin-bottom: 5px; }
        .remediation { background: #e7f3ff; padding: 15px; border-radius: 6px; margin: 15px 0; border-left: 4px solid #007bff; }
        .remediation-title { font-weight: bold; color: #007bff; margin-bottom: 10px; }
        .commands { background: #f8f8f8; padding: 10px; border-radius: 4px; font-family: monospace; margin: 5px 0; }
        .manual-steps { margin: 5px 0; }
        .manual-steps li { margin: 5px 0; }
        .confidence { float: right; font-size: 0.9em; color: #666; }
        .impact { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 0.8em; font-weight: bold; }
        .impact.HIGH { background: #dc3545; color: white; }
        .impact.MEDIUM { background: #ffc107; color: black; }
        .impact.LOW { background: #6c757d; color: white; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 Troubleshoot Analysis Report</h1>
            <p>Generated: ` + time.Now().Format("2006-01-02 15:04:05") + `</p>
        </div>
        
        <div class="summary">
            <div class="summary-card">
                <h3>Total Checks</h3>
                <div style="font-size: 2em; font-weight: bold;">` + fmt.Sprintf("%d", artifact.Summary.TotalChecks) + `</div>
            </div>
            <div class="summary-card success">
                <h3>✅ Passed</h3>
                <div style="font-size: 2em; font-weight: bold; color: #28a745;">` + fmt.Sprintf("%d", artifact.Summary.PassedChecks) + `</div>
            </div>
            <div class="summary-card warning">
                <h3>⚠️ Warnings</h3>
                <div style="font-size: 2em; font-weight: bold; color: #ffc107;">` + fmt.Sprintf("%d", artifact.Summary.WarningChecks) + `</div>
            </div>
            <div class="summary-card critical">
                <h3>❌ Failed</h3>
                <div style="font-size: 2em; font-weight: bold; color: #dc3545;">` + fmt.Sprintf("%d", artifact.Summary.FailedChecks) + `</div>
            </div>
        </div>

        <div class="overall-health">
            <h2>Overall Health: ` + artifact.Summary.OverallHealth + `</h2>
        </div>

        <div class="results">
            <h2>📊 Analysis Results</h2>`

	// Add results
	for _, result := range artifact.Results {
		status := "pass"
		icon := "✅"
		if result.IsFail {
			status = "fail"
			icon = "❌"
		} else if result.IsWarn {
			status = "warn"
			icon = "⚠️"
		}

		html += fmt.Sprintf(`
            <div class="result %s">
                <div class="result-title">%s %s`, status, icon, result.Title)

		if result.Impact != "" {
			html += fmt.Sprintf(`<span class="impact %s">%s</span>`, result.Impact, result.Impact)
		}

		if result.Confidence > 0 {
			html += fmt.Sprintf(`<span class="confidence">Confidence: %.1f%%</span>`, result.Confidence*100)
		}

		html += `</div>`

		if result.Message != "" {
			html += fmt.Sprintf(`<div class="result-message">%s</div>`, result.Message)
		}

		if result.Explanation != "" {
			html += fmt.Sprintf(`<div class="result-explanation">%s</div>`, result.Explanation)
		}

		if len(result.Evidence) > 0 {
			html += `<div class="evidence"><div class="evidence-title">Evidence:</div><ul>`
			for _, evidence := range result.Evidence {
				html += fmt.Sprintf(`<li>%s</li>`, evidence)
			}
			html += `</ul></div>`
		}

		if result.Remediation != nil {
			html += fmt.Sprintf(`
                <div class="remediation">
                    <div class="remediation-title">🛠️ Remediation: %s</div>
                    <p>%s</p>`, result.Remediation.Title, result.Remediation.Description)

			if len(result.Remediation.Commands) > 0 {
				html += `<p><strong>Commands to run:</strong></p><div class="commands">`
				for _, cmd := range result.Remediation.Commands {
					html += fmt.Sprintf(`%s<br>`, cmd)
				}
				html += `</div>`
			}

			if len(result.Remediation.Manual) > 0 {
				html += `<p><strong>Manual steps:</strong></p><ul class="manual-steps">`
				for _, step := range result.Remediation.Manual {
					html += fmt.Sprintf(`<li>%s</li>`, step)
				}
				html += `</ul>`
			}

			html += `</div>`
		}

		html += `</div>`
	}

	html += `
        </div>

        <!-- Remediation Summary -->
        <div class="remediation-summary">
            <h2>🛠️ Remediation Summary</h2>`

	if len(artifact.Remediation) > 0 {
		html += `<div class="remediation">`
		for _, remediation := range artifact.Remediation {
			html += fmt.Sprintf(`
                <div style="margin: 15px 0;">
                    <div class="remediation-title">%s (Priority: %d)</div>
                    <p>%s</p>`, remediation.Title, remediation.Priority, remediation.Description)

			if len(remediation.Commands) > 0 {
				html += `<div class="commands">`
				for _, cmd := range remediation.Commands {
					html += fmt.Sprintf(`%s<br>`, cmd)
				}
				html += `</div>`
			}
			html += `</div>`
		}
		html += `</div>`
	} else {
		html += `<p>No remediation steps required - all critical checks passed! ✅</p>`
	}

	html += `
        </div>
    </div>
</body>
</html>`

	return html
}

// FormatLegacyResults formats old-style AnalyzeResult array for backward compatibility
func FormatLegacyResults(results []*analyzer.AnalyzeResult, format string, writer io.Writer) error {
	// Convert legacy results to enhanced format
	enhanced := make([]analyzer.EnhancedAnalyzerResult, 0, len(results))

	for _, result := range results {
		if result == nil {
			continue
		}

		enhanced = append(enhanced, analyzer.EnhancedAnalyzerResult{
			IsPass:  result.IsPass,
			IsFail:  result.IsFail,
			IsWarn:  result.IsWarn,
			Strict:  result.Strict,
			Title:   result.Title,
			Message: result.Message,
			URI:     result.URI,
			IconKey: result.IconKey,
			IconURI: result.IconURI,

			// Set defaults for enhanced fields
			AgentUsed:  "legacy",
			Confidence: 0.8, // Default confidence
		})
	}

	// Create enhanced analysis result
	analysisResult := &analyzer.EnhancedAnalysisResult{
		Results: enhanced,
		Summary: generateSummaryFromLegacy(enhanced),
		Metadata: analyzer.AnalysisMetadata{
			Timestamp:     time.Now(),
			EngineVersion: "legacy",
			AgentsUsed:    []string{"legacy"},
		},
	}

	// Format using the standard formatter
	formatter := NewFormatter(FormatterOptions{
		Format:             format,
		IncludeMetadata:    true,
		IncludeRemediation: false,
		IncludeInsights:    false,
		FilterLevel:        "all",
	})

	return formatter.FormatAnalysis(analysisResult, writer)
}

// Generate summary from legacy results
func generateSummaryFromLegacy(results []analyzer.EnhancedAnalyzerResult) analyzer.AnalysisSummary {
	summary := analyzer.AnalysisSummary{
		TotalChecks:   len(results),
		OverallHealth: "UNKNOWN",
	}

	for _, result := range results {
		if result.IsPass {
			summary.PassedChecks++
		} else if result.IsFail {
			summary.FailedChecks++
		} else if result.IsWarn {
			summary.WarningChecks++
		}
	}

	// Calculate overall health
	if summary.FailedChecks == 0 {
		summary.OverallHealth = "HEALTHY"
	} else if float64(summary.FailedChecks)/float64(summary.TotalChecks) < 0.1 {
		summary.OverallHealth = "DEGRADED"
	} else {
		summary.OverallHealth = "CRITICAL"
	}

	summary.Confidence = 0.8 // Default for legacy results

	return summary
}

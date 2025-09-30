package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

// JSONFormatter formats analysis results as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func (f *JSONFormatter) ContentType() string {
	return "application/json"
}

func (f *JSONFormatter) FileExtension() string {
	return "json"
}

// YAMLFormatter formats analysis results as YAML
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error) {
	return yaml.Marshal(result)
}

func (f *YAMLFormatter) ContentType() string {
	return "application/x-yaml"
}

func (f *YAMLFormatter) FileExtension() string {
	return "yaml"
}

// HTMLFormatter formats analysis results as HTML
type HTMLFormatter struct{}

func (f *HTMLFormatter) Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error) {
	tmpl := template.New("analysis").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"statusIcon": func(r *analyzer.AnalyzerResult) string {
			if r.IsPass {
				return "✅"
			} else if r.IsWarn {
				return "⚠️"
			} else if r.IsFail {
				return "❌"
			}
			return "❓"
		},
		"statusClass": func(r *analyzer.AnalyzerResult) string {
			if r.IsPass {
				return "success"
			} else if r.IsWarn {
				return "warning"
			} else if r.IsFail {
				return "danger"
			}
			return "info"
		},
		"priorityBadge": func(priority int) string {
			if priority >= 8 {
				return "badge-danger"
			} else if priority >= 5 {
				return "badge-warning"
			}
			return "badge-info"
		},
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
	})

	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Analysis Report</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <style>
        .analysis-header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; }
        .summary-card { border-left: 4px solid #007bff; }
        .results-table { box-shadow: 0 0.125rem 0.25rem rgba(0, 0, 0, 0.075); }
        .remediation-card { background-color: #f8f9fa; }
        .insight-badge { font-size: 0.8em; }
        .agent-info { font-size: 0.9em; color: #6c757d; }
        .correlation-item { border-left: 3px solid #28a745; padding-left: 10px; margin-bottom: 10px; }
    </style>
</head>
<body>
    <div class="container-fluid">
        <!-- Header -->
        <div class="row analysis-header py-4 mb-4">
            <div class="col">
                <h1 class="h2 mb-2">Troubleshoot Analysis Report</h1>
                <p class="mb-0">Generated on {{formatTime .Metadata.Timestamp}} | Engine Version {{.Metadata.EngineVersion}}</p>
            </div>
        </div>

        <!-- Summary Cards -->
        <div class="row mb-4">
            <div class="col-md-3">
                <div class="card summary-card">
                    <div class="card-body text-center">
                        <h5 class="card-title text-success">{{.Summary.PassCount}}</h5>
                        <p class="card-text">Passed</p>
                    </div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card summary-card">
                    <div class="card-body text-center">
                        <h5 class="card-title text-warning">{{.Summary.WarnCount}}</h5>
                        <p class="card-text">Warnings</p>
                    </div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card summary-card">
                    <div class="card-body text-center">
                        <h5 class="card-title text-danger">{{.Summary.FailCount}}</h5>
                        <p class="card-text">Failed</p>
                    </div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card summary-card">
                    <div class="card-body text-center">
                        <h5 class="card-title">{{.Summary.TotalAnalyzers}}</h5>
                        <p class="card-text">Total Analyzers</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Analysis Details -->
        <div class="row mb-4">
            <div class="col-md-8">
                <div class="card">
                    <div class="card-header">
                        <h5 class="mb-0">Analysis Results</h5>
                    </div>
                    <div class="card-body">
                        <div class="table-responsive">
                            <table class="table table-hover results-table">
                                <thead class="table-light">
                                    <tr>
                                        <th>Status</th>
                                        <th>Title</th>
                                        <th>Category</th>
                                        <th>Agent</th>
                                        <th>Confidence</th>
                                        <th>Message</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {{range .Results}}
                                    <tr class="table-{{statusClass .}}">
                                        <td>{{statusIcon .}}</td>
                                        <td><strong>{{.Title}}</strong></td>
                                        <td><span class="badge bg-secondary">{{.Category}}</span></td>
                                        <td><span class="agent-info">{{.AgentName}}</span></td>
                                        <td>{{if .Confidence}}{{printf "%.1f%%" (mul .Confidence 100)}}{{else}}-{{end}}</td>
                                        <td>{{truncate .Message 100}}</td>
                                    </tr>
                                    {{end}}
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>
            </div>
            
            <div class="col-md-4">
                <!-- Agent Information -->
                <div class="card mb-3">
                    <div class="card-header">
                        <h6 class="mb-0">Agents Used</h6>
                    </div>
                    <div class="card-body">
                        {{range .Metadata.Agents}}
                        <div class="d-flex justify-content-between align-items-center mb-2">
                            <span><strong>{{.Name}}</strong></span>
                            <span class="text-muted">{{.Duration}}</span>
                        </div>
                        <div class="text-muted small mb-2">
                            {{.ResultCount}} results, {{.ErrorCount}} errors
                        </div>
                        <hr>
                        {{end}}
                    </div>
                </div>

                <!-- Summary Stats -->
                <div class="card">
                    <div class="card-header">
                        <h6 class="mb-0">Summary Statistics</h6>
                    </div>
                    <div class="card-body">
                        <p><strong>Duration:</strong> {{.Summary.Duration}}</p>
                        {{if .Summary.Confidence}}<p><strong>Confidence:</strong> {{printf "%.1f%%" (mul .Summary.Confidence 100)}}</p>{{end}}
                        <p><strong>Agents:</strong> {{len .Summary.AgentsUsed}}</p>
                        <p><strong>Errors:</strong> {{len .Errors}}</p>
                    </div>
                </div>
            </div>
        </div>

        {{if .Remediation}}
        <!-- Remediation Steps -->
        <div class="row mb-4">
            <div class="col">
                <div class="card">
                    <div class="card-header">
                        <h5 class="mb-0">Remediation Steps</h5>
                    </div>
                    <div class="card-body">
                        {{range .Remediation}}
                        <div class="card remediation-card mb-3">
                            <div class="card-body">
                                <div class="d-flex justify-content-between align-items-start mb-2">
                                    <h6 class="card-title">{{.Description}}</h6>
                                    <span class="badge {{priorityBadge .Priority}}">Priority {{.Priority}}</span>
                                </div>
                                {{if .Command}}<p class="text-muted"><code>{{.Command}}</code></p>{{end}}
                                <div class="d-flex justify-content-between align-items-center">
                                    <span class="badge bg-info">{{.Category}}</span>
                                    {{if .IsAutomatable}}<span class="badge bg-success">Automatable</span>{{end}}
                                </div>
                                {{if .Documentation}}<p class="mt-2"><a href="{{.Documentation}}" target="_blank">Documentation</a></p>{{end}}
                            </div>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        {{if .Metadata.Correlations}}
        <!-- Correlations -->
        <div class="row mb-4">
            <div class="col">
                <div class="card">
                    <div class="card-header">
                        <h5 class="mb-0">Correlations and Insights</h5>
                    </div>
                    <div class="card-body">
                        {{range .Metadata.Correlations}}
                        <div class="correlation-item">
                            <h6>{{.Type}}</h6>
                            <p>{{.Description}}</p>
                            <small class="text-muted">Confidence: {{printf "%.1f%%" (mul .Confidence 100)}}</small>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        {{if .Errors}}
        <!-- Errors -->
        <div class="row mb-4">
            <div class="col">
                <div class="card border-danger">
                    <div class="card-header text-danger">
                        <h5 class="mb-0">Analysis Errors</h5>
                    </div>
                    <div class="card-body">
                        {{range .Errors}}
                        <div class="alert alert-danger">
                            <strong>{{.Agent}}{{if .Analyzer}} - {{.Analyzer}}{{end}}:</strong> {{.Error}}
                            <br><small class="text-muted">{{formatTime .Timestamp}}</small>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- Footer -->
        <footer class="text-center text-muted py-3">
            <p>Generated by Troubleshoot Analysis Engine v{{.Metadata.EngineVersion}}</p>
        </footer>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>`

	t, err := tmpl.Parse(htmlTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse HTML template")
	}

	var buf strings.Builder
	if err := t.Execute(&buf, result); err != nil {
		return nil, errors.Wrap(err, "failed to execute HTML template")
	}

	return []byte(buf.String()), nil
}

func (f *HTMLFormatter) ContentType() string {
	return "text/html"
}

func (f *HTMLFormatter) FileExtension() string {
	return "html"
}

// TextFormatter formats analysis results as plain text
type TextFormatter struct{}

func (f *TextFormatter) Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error) {
	var builder strings.Builder

	// Header
	builder.WriteString("TROUBLESHOOT ANALYSIS REPORT\n")
	builder.WriteString("===========================\n\n")

	// Timestamp
	builder.WriteString(fmt.Sprintf("Generated: %s\n", result.Metadata.Timestamp.Format("2006-01-02 15:04:05")))
	builder.WriteString(fmt.Sprintf("Engine Version: %s\n", result.Metadata.EngineVersion))
	builder.WriteString(fmt.Sprintf("Duration: %s\n\n", result.Summary.Duration))

	// Summary
	builder.WriteString("SUMMARY\n")
	builder.WriteString("-------\n")
	builder.WriteString(fmt.Sprintf("Total Analyzers: %d\n", result.Summary.TotalAnalyzers))
	builder.WriteString(fmt.Sprintf("Passed: %d\n", result.Summary.PassCount))
	builder.WriteString(fmt.Sprintf("Warnings: %d\n", result.Summary.WarnCount))
	builder.WriteString(fmt.Sprintf("Failed: %d\n", result.Summary.FailCount))
	builder.WriteString(fmt.Sprintf("Errors: %d\n", result.Summary.ErrorCount))

	if result.Summary.Confidence > 0 {
		builder.WriteString(fmt.Sprintf("Confidence: %.1f%%\n", result.Summary.Confidence*100))
	}

	builder.WriteString(fmt.Sprintf("Agents Used: %s\n\n", strings.Join(result.Summary.AgentsUsed, ", ")))

	// Results
	builder.WriteString("ANALYSIS RESULTS\n")
	builder.WriteString("----------------\n\n")

	// Group results by status
	var passResults, warnResults, failResults []*analyzer.AnalyzerResult
	for _, r := range result.Results {
		if r.IsPass {
			passResults = append(passResults, r)
		} else if r.IsWarn {
			warnResults = append(warnResults, r)
		} else if r.IsFail {
			failResults = append(failResults, r)
		}
	}

	// Failed results first
	if len(failResults) > 0 {
		builder.WriteString("FAILED CHECKS:\n")
		for _, r := range failResults {
			f.writeResultText(&builder, r, "❌")
		}
		builder.WriteString("\n")
	}

	// Warning results
	if len(warnResults) > 0 {
		builder.WriteString("WARNING CHECKS:\n")
		for _, r := range warnResults {
			f.writeResultText(&builder, r, "⚠️")
		}
		builder.WriteString("\n")
	}

	// Passed results (summary only to save space)
	if len(passResults) > 0 {
		builder.WriteString(fmt.Sprintf("PASSED CHECKS: %d checks passed\n\n", len(passResults)))
	}

	// Remediation steps
	if len(result.Remediation) > 0 {
		builder.WriteString("REMEDIATION STEPS\n")
		builder.WriteString("-----------------\n\n")

		// Sort by priority
		remediation := make([]analyzer.RemediationStep, len(result.Remediation))
		copy(remediation, result.Remediation)
		sort.Slice(remediation, func(i, j int) bool {
			return remediation[i].Priority > remediation[j].Priority
		})

		for i, step := range remediation {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, step.Description))
			builder.WriteString(fmt.Sprintf("   Category: %s | Priority: %d", step.Category, step.Priority))
			if step.IsAutomatable {
				builder.WriteString(" | Automatable")
			}
			builder.WriteString("\n")

			if step.Command != "" {
				builder.WriteString(fmt.Sprintf("   Command: %s\n", step.Command))
			}

			if step.Documentation != "" {
				builder.WriteString(fmt.Sprintf("   Documentation: %s\n", step.Documentation))
			}

			builder.WriteString("\n")
		}
	}

	// Agent information
	if len(result.Metadata.Agents) > 0 {
		builder.WriteString("AGENT INFORMATION\n")
		builder.WriteString("-----------------\n")

		for _, agent := range result.Metadata.Agents {
			builder.WriteString(fmt.Sprintf("Agent: %s\n", agent.Name))
			builder.WriteString(fmt.Sprintf("  Duration: %s\n", agent.Duration))
			builder.WriteString(fmt.Sprintf("  Results: %d\n", agent.ResultCount))
			builder.WriteString(fmt.Sprintf("  Errors: %d\n", agent.ErrorCount))
			builder.WriteString(fmt.Sprintf("  Capabilities: %s\n\n", strings.Join(agent.Capabilities, ", ")))
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		builder.WriteString("ANALYSIS ERRORS\n")
		builder.WriteString("---------------\n")

		for _, err := range result.Errors {
			builder.WriteString(fmt.Sprintf("• %s", err.Error))
			if err.Agent != "" {
				builder.WriteString(fmt.Sprintf(" (Agent: %s)", err.Agent))
			}
			if err.Analyzer != "" {
				builder.WriteString(fmt.Sprintf(" (Analyzer: %s)", err.Analyzer))
			}
			builder.WriteString(fmt.Sprintf(" [%s]\n", err.Timestamp.Format("15:04:05")))
		}
		builder.WriteString("\n")
	}

	return []byte(builder.String()), nil
}

func (f *TextFormatter) writeResultText(builder *strings.Builder, result *analyzer.AnalyzerResult, icon string) {
	builder.WriteString(fmt.Sprintf("%s %s", icon, result.Title))
	if result.Category != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", result.Category))
	}
	builder.WriteString("\n")

	builder.WriteString(fmt.Sprintf("   %s", result.Message))
	if result.AgentName != "" {
		builder.WriteString(fmt.Sprintf(" (via %s)", result.AgentName))
	}
	if result.Confidence > 0 {
		builder.WriteString(fmt.Sprintf(" [%.0f%% confidence]", result.Confidence*100))
	}
	builder.WriteString("\n")

	if len(result.Insights) > 0 {
		builder.WriteString("   Insights:\n")
		for _, insight := range result.Insights {
			builder.WriteString(fmt.Sprintf("   • %s\n", insight))
		}
	}

	if result.Remediation != nil {
		builder.WriteString(fmt.Sprintf("   Remediation: %s\n", result.Remediation.Description))
		if result.Remediation.Command != "" {
			builder.WriteString(fmt.Sprintf("   Command: %s\n", result.Remediation.Command))
		}
	}

	builder.WriteString("\n")
}

func (f *TextFormatter) ContentType() string {
	return "text/plain"
}

func (f *TextFormatter) FileExtension() string {
	return "txt"
}

package remediation

import (
	"time"
)

// RemediationStep represents a single remediation action
type RemediationStep struct {
	ID            string                `json:"id"`
	Title         string                `json:"title"`
	Description   string                `json:"description"`
	Category      RemediationCategory   `json:"category"`
	Priority      RemediationPriority   `json:"priority"`
	Impact        RemediationImpact     `json:"impact"`
	Difficulty    RemediationDifficulty `json:"difficulty"`
	EstimatedTime time.Duration         `json:"estimated_time"`
	Command       *CommandStep          `json:"command,omitempty"`
	Manual        *ManualStep           `json:"manual,omitempty"`
	Script        *ScriptStep           `json:"script,omitempty"`
	Dependencies  []string              `json:"dependencies,omitempty"`
	Prerequisites []string              `json:"prerequisites,omitempty"`
	Verification  *VerificationStep     `json:"verification,omitempty"`
	Rollback      *RollbackStep         `json:"rollback,omitempty"`
	Documentation []DocumentationLink   `json:"documentation,omitempty"`
	Tags          []string              `json:"tags,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// RemediationCategory defines the type of remediation
type RemediationCategory string

const (
	CategoryConfiguration  RemediationCategory = "configuration"
	CategoryResource       RemediationCategory = "resource"
	CategorySecurity       RemediationCategory = "security"
	CategoryNetwork        RemediationCategory = "network"
	CategoryStorage        RemediationCategory = "storage"
	CategoryApplication    RemediationCategory = "application"
	CategoryInfrastructure RemediationCategory = "infrastructure"
	CategoryMonitoring     RemediationCategory = "monitoring"
	CategoryCustom         RemediationCategory = "custom"
)

// RemediationPriority defines the urgency of remediation
type RemediationPriority string

const (
	PriorityCritical RemediationPriority = "critical"
	PriorityHigh     RemediationPriority = "high"
	PriorityMedium   RemediationPriority = "medium"
	PriorityLow      RemediationPriority = "low"
	PriorityInfo     RemediationPriority = "info"
)

// RemediationImpact defines the expected impact of the remediation
type RemediationImpact string

const (
	ImpactHigh    RemediationImpact = "high"    // Significant improvement expected
	ImpactMedium  RemediationImpact = "medium"  // Moderate improvement expected
	ImpactLow     RemediationImpact = "low"     // Minor improvement expected
	ImpactUnknown RemediationImpact = "unknown" // Impact uncertain
)

// RemediationDifficulty defines how difficult the remediation is to execute
type RemediationDifficulty string

const (
	DifficultyEasy     RemediationDifficulty = "easy"     // Single command or simple change
	DifficultyModerate RemediationDifficulty = "moderate" // Multiple steps, some complexity
	DifficultyHard     RemediationDifficulty = "hard"     // Complex changes, multiple systems
	DifficultyExpert   RemediationDifficulty = "expert"   // Requires deep expertise
)

// CommandStep represents a command-line remediation step
type CommandStep struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Sudo        bool              `json:"sudo,omitempty"`
	Namespace   string            `json:"namespace,omitempty"` // For Kubernetes commands
}

// ManualStep represents a manual remediation step
type ManualStep struct {
	Instructions []string         `json:"instructions"`
	Checklist    []ChecklistItem  `json:"checklist,omitempty"`
	Images       []ImageReference `json:"images,omitempty"`
	Notes        []string         `json:"notes,omitempty"`
}

// ScriptStep represents a script-based remediation step
type ScriptStep struct {
	Language    ScriptLanguage    `json:"language"`
	Content     string            `json:"content,omitempty"`
	FilePath    string            `json:"file_path,omitempty"`
	Arguments   []string          `json:"arguments,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
}

// VerificationStep represents how to verify the remediation was successful
type VerificationStep struct {
	Command        *CommandStep  `json:"command,omitempty"`
	ExpectedOutput string        `json:"expected_output,omitempty"`
	Manual         []string      `json:"manual,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
}

// RollbackStep represents how to rollback the remediation if needed
type RollbackStep struct {
	Command     *CommandStep `json:"command,omitempty"`
	Manual      []string     `json:"manual,omitempty"`
	Description string       `json:"description"`
}

// ChecklistItem represents a single checklist item in a manual step
type ChecklistItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
}

// ImageReference represents a reference to an image or diagram
type ImageReference struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	AltText     string `json:"alt_text,omitempty"`
}

// DocumentationLink represents a link to relevant documentation
type DocumentationLink struct {
	Title       string            `json:"title"`
	URL         string            `json:"url"`
	Description string            `json:"description,omitempty"`
	Type        DocumentationType `json:"type"`
}

// ScriptLanguage defines the scripting language
type ScriptLanguage string

const (
	LanguageBash       ScriptLanguage = "bash"
	LanguagePowerShell ScriptLanguage = "powershell"
	LanguagePython     ScriptLanguage = "python"
	LanguageGo         ScriptLanguage = "go"
	LanguageYAML       ScriptLanguage = "yaml"
	LanguageJSON       ScriptLanguage = "json"
)

// DocumentationType defines the type of documentation
type DocumentationType string

const (
	DocTypeOfficial     DocumentationType = "official"     // Official product documentation
	DocTypeCommunity    DocumentationType = "community"    // Community-contributed content
	DocTypeTutorial     DocumentationType = "tutorial"     // Step-by-step tutorials
	DocTypeReference    DocumentationType = "reference"    // Reference materials
	DocTypeTroubleshoot DocumentationType = "troubleshoot" // Troubleshooting guides
)

// RemediationPlan represents a collection of remediation steps
type RemediationPlan struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	Steps        []RemediationStep     `json:"steps"`
	TotalTime    time.Duration         `json:"total_time"`
	Priority     RemediationPriority   `json:"priority"`
	Impact       RemediationImpact     `json:"impact"`
	Categories   []RemediationCategory `json:"categories"`
	Dependencies []string              `json:"dependencies,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// RemediationResult represents the result of executing a remediation step
type RemediationResult struct {
	StepID     string            `json:"step_id"`
	Status     RemediationStatus `json:"status"`
	Output     string            `json:"output,omitempty"`
	Error      string            `json:"error,omitempty"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Duration   time.Duration     `json:"duration"`
	Verified   bool              `json:"verified"`
	RolledBack bool              `json:"rolled_back"`
}

// RemediationStatus defines the execution status of a remediation step
type RemediationStatus string

const (
	StatusPending    RemediationStatus = "pending"
	StatusRunning    RemediationStatus = "running"
	StatusCompleted  RemediationStatus = "completed"
	StatusFailed     RemediationStatus = "failed"
	StatusSkipped    RemediationStatus = "skipped"
	StatusRolledBack RemediationStatus = "rolled_back"
)

// RemediationExecution represents the execution of a remediation plan
type RemediationExecution struct {
	ID        string              `json:"id"`
	PlanID    string              `json:"plan_id"`
	Status    RemediationStatus   `json:"status"`
	Results   []RemediationResult `json:"results"`
	StartTime time.Time           `json:"start_time"`
	EndTime   time.Time           `json:"end_time"`
	Duration  time.Duration       `json:"duration"`
	Error     string              `json:"error,omitempty"`
}

// RemediationContext provides context for remediation generation
type RemediationContext struct {
	AnalysisResults []AnalysisResult   `json:"analysis_results"`
	Environment     EnvironmentContext `json:"environment"`
	UserPreferences UserPreferences    `json:"user_preferences"`
	Constraints     []Constraint       `json:"constraints"`
	HistoricalData  *HistoricalContext `json:"historical_data,omitempty"`
}

// AnalysisResult represents an analysis result (placeholder for integration)
type AnalysisResult struct {
	ID          string      `json:"id"`
	AnalyzerID  string      `json:"analyzer_id"`
	Title       string      `json:"title"`
	Severity    string      `json:"severity"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Evidence    []string    `json:"evidence"`
	Data        interface{} `json:"data"`
}

// EnvironmentContext provides context about the target environment
type EnvironmentContext struct {
	Platform      string            `json:"platform"`       // kubernetes, docker, bare-metal
	Version       string            `json:"version"`        // Kubernetes version, etc.
	CloudProvider string            `json:"cloud_provider"` // aws, azure, gcp, on-premises
	Namespace     string            `json:"namespace,omitempty"`
	Cluster       string            `json:"cluster,omitempty"`
	Region        string            `json:"region,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
	Capabilities  []string          `json:"capabilities"` // Available tools/features
	Restrictions  []string          `json:"restrictions"` // Security/policy restrictions
}

// UserPreferences defines user preferences for remediation
type UserPreferences struct {
	MaxRisk           RemediationImpact     `json:"max_risk"`           // Maximum acceptable risk
	PreferredMethods  []RemediationCategory `json:"preferred_methods"`  // Preferred remediation types
	AutoExecute       bool                  `json:"auto_execute"`       // Allow automatic execution
	NotificationLevel RemediationPriority   `json:"notification_level"` // Minimum priority for notifications
	TimeConstraints   *TimeConstraints      `json:"time_constraints,omitempty"`
	SkillLevel        SkillLevel            `json:"skill_level"` // User's technical skill level
}

// TimeConstraints defines time-based constraints
type TimeConstraints struct {
	MaintenanceWindows []MaintenanceWindow `json:"maintenance_windows,omitempty"`
	MaxDuration        time.Duration       `json:"max_duration,omitempty"`
	Deadline           *time.Time          `json:"deadline,omitempty"`
}

// MaintenanceWindow defines when remediation can be performed
type MaintenanceWindow struct {
	Start     time.Time      `json:"start"`
	End       time.Time      `json:"end"`
	Timezone  string         `json:"timezone"`
	Recurring bool           `json:"recurring"`
	Days      []time.Weekday `json:"days,omitempty"`
}

// SkillLevel defines the user's technical skill level
type SkillLevel string

const (
	SkillBeginner     SkillLevel = "beginner"     // Basic command-line knowledge
	SkillIntermediate SkillLevel = "intermediate" // Comfortable with system administration
	SkillAdvanced     SkillLevel = "advanced"     // Expert-level technical skills
	SkillExpert       SkillLevel = "expert"       // Deep domain expertise
)

// Constraint represents a constraint on remediation execution
type Constraint struct {
	Type        ConstraintType `json:"type"`
	Description string         `json:"description"`
	Value       interface{}    `json:"value"`
	Severity    string         `json:"severity"`
}

// ConstraintType defines the type of constraint
type ConstraintType string

const (
	ConstraintSecurity   ConstraintType = "security"   // Security policies
	ConstraintCompliance ConstraintType = "compliance" // Compliance requirements
	ConstraintBudget     ConstraintType = "budget"     // Cost constraints
	ConstraintTime       ConstraintType = "time"       // Time constraints
	ConstraintResource   ConstraintType = "resource"   // Resource constraints
	ConstraintTechnical  ConstraintType = "technical"  // Technical limitations
)

// HistoricalContext provides historical data for trend analysis
type HistoricalContext struct {
	PreviousAnalyses     []HistoricalAnalysis    `json:"previous_analyses"`
	PreviousRemediations []HistoricalRemediation `json:"previous_remediations"`
	Trends               []Trend                 `json:"trends"`
	Baselines            map[string]interface{}  `json:"baselines"`
}

// HistoricalAnalysis represents a previous analysis result
type HistoricalAnalysis struct {
	Timestamp time.Time        `json:"timestamp"`
	Results   []AnalysisResult `json:"results"`
	Summary   AnalysisSummary  `json:"summary"`
}

// HistoricalRemediation represents a previous remediation execution
type HistoricalRemediation struct {
	Timestamp time.Time            `json:"timestamp"`
	Execution RemediationExecution `json:"execution"`
	Success   bool                 `json:"success"`
	Notes     string               `json:"notes,omitempty"`
}

// Trend represents a trend in analysis or remediation data
type Trend struct {
	Category    string         `json:"category"`
	Metric      string         `json:"metric"`
	Direction   TrendDirection `json:"direction"`
	Magnitude   float64        `json:"magnitude"`
	Confidence  float64        `json:"confidence"`
	TimeRange   TimeRange      `json:"time_range"`
	Description string         `json:"description"`
}

// TrendDirection defines the direction of a trend
type TrendDirection string

const (
	TrendImproving TrendDirection = "improving"
	TrendStable    TrendDirection = "stable"
	TrendDegrading TrendDirection = "degrading"
	TrendVolatile  TrendDirection = "volatile"
	TrendUnknown   TrendDirection = "unknown"
)

// TimeRange defines a time range for trend analysis
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// AnalysisSummary provides a summary of analysis results
type AnalysisSummary struct {
	TotalIssues      int            `json:"total_issues"`
	IssuesBySeverity map[string]int `json:"issues_by_severity"`
	IssuesByCategory map[string]int `json:"issues_by_category"`
	OverallHealth    string         `json:"overall_health"`
	Recommendations  int            `json:"recommendations"`
}

// RemediationMetadata provides metadata about remediation capabilities
type RemediationMetadata struct {
	Version             string                `json:"version"`
	SupportedCategories []RemediationCategory `json:"supported_categories"`
	SupportedLanguages  []ScriptLanguage      `json:"supported_languages"`
	AvailableProviders  []RemediationProvider `json:"available_providers"`
	Capabilities        []string              `json:"capabilities"`
}

// RemediationProvider represents a provider of remediation suggestions
type RemediationProvider struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Version     string                `json:"version"`
	Categories  []RemediationCategory `json:"categories"`
	Enabled     bool                  `json:"enabled"`
}

// RemediationTemplate represents a template for generating remediations
type RemediationTemplate struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    RemediationCategory `json:"category"`
	Pattern     string              `json:"pattern"`    // Pattern to match against analysis results
	Template    RemediationStep     `json:"template"`   // Template step
	Variables   []TemplateVariable  `json:"variables"`  // Variables that can be substituted
	Conditions  []TemplateCondition `json:"conditions"` // Conditions for applying the template
}

// TemplateVariable represents a variable in a remediation template
type TemplateVariable struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"` // string, number, boolean, list
	Description  string      `json:"description"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Required     bool        `json:"required"`
	Pattern      string      `json:"pattern,omitempty"` // Validation pattern for strings
}

// TemplateCondition represents a condition for applying a remediation template
type TemplateCondition struct {
	Field    string      `json:"field"`    // Field to check in analysis result
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, matches
	Value    interface{} `json:"value"`    // Value to compare against
	Required bool        `json:"required"` // Whether this condition must be met
}

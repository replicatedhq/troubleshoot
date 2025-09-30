package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Job represents a scheduled support bundle collection job
type Job struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"` // Cron expression
	Namespace string    `json:"namespace"`
	Auto      bool      `json:"auto"` // Auto-discovery
	Upload    string    `json:"upload,omitempty"`
	Enabled   bool      `json:"enabled"`
	RunCount  int       `json:"runCount"`
	LastRun   time.Time `json:"lastRun,omitempty"`
	Created   time.Time `json:"created"`
}

// Manager handles job operations
type Manager struct {
	storageDir string
}

// NewManager creates a new job manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, ".troubleshoot", "scheduled-jobs")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory %s: %w", storageDir, err)
	}

	return &Manager{storageDir: storageDir}, nil
}

// CreateJob creates a new scheduled job
func (m *Manager) CreateJob(name, schedule, namespace string, auto bool, upload string) (*Job, error) {
	// Input validation
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("job name cannot be empty")
	}

	// Sanitize job name for filesystem safety
	name = strings.TrimSpace(name)
	if len(name) > 100 {
		return nil, fmt.Errorf("job name too long, maximum 100 characters")
	}

	// Check for invalid filename characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return nil, fmt.Errorf("job name contains invalid character: %s", char)
		}
	}

	// Cron validation - check it has 5 parts and basic field validation
	if err := validateCronSchedule(schedule); err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	job := &Job{
		ID:        generateJobID(),
		Name:      name,
		Schedule:  schedule,
		Namespace: namespace,
		Auto:      auto,
		Upload:    upload,
		Enabled:   true,
		Created:   time.Now(),
	}

	if err := m.saveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// ListJobs returns all saved jobs
func (m *Manager) ListJobs() ([]*Job, error) {
	files, err := filepath.Glob(filepath.Join(m.storageDir, "*.json"))
	if err != nil {
		return nil, err
	}

	var jobs []*Job
	for _, file := range files {
		job, err := m.loadJobFromFile(file)
		if err != nil {
			continue // Skip invalid files
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJob retrieves a job by name or ID
func (m *Manager) GetJob(nameOrID string) (*Job, error) {
	jobs, err := m.ListJobs()
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if job.Name == nameOrID || job.ID == nameOrID {
			return job, nil
		}
	}

	return nil, fmt.Errorf("job not found: %s", nameOrID)
}

// DeleteJob removes a job
func (m *Manager) DeleteJob(nameOrID string) error {
	job, err := m.GetJob(nameOrID)
	if err != nil {
		return err
	}

	jobFile := filepath.Join(m.storageDir, job.ID+".json")
	return os.Remove(jobFile)
}

// saveJob saves a job to a JSON file
func (m *Manager) saveJob(job *Job) error {
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}

	jobFile := filepath.Join(m.storageDir, job.ID+".json")
	return os.WriteFile(jobFile, data, 0644)
}

// loadJobFromFile loads a job from a JSON file
func (m *Manager) loadJobFromFile(filename string) (*Job, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var job Job
	err = json.Unmarshal(data, &job)
	return &job, err
}

// validateCronSchedule performs basic cron schedule validation
func validateCronSchedule(schedule string) error {
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return fmt.Errorf("expected 5 fields (minute hour day-of-month month day-of-week), got %d", len(parts))
	}

	// Validate each field has reasonable values
	fieldNames := []string{"minute", "hour", "day-of-month", "month", "day-of-week"}
	fieldRanges := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}

	for i, field := range parts {
		if err := validateCronField(field, fieldRanges[i][0], fieldRanges[i][1], fieldNames[i]); err != nil {
			return err
		}
	}

	return nil
}

// validateCronField validates a single cron field
func validateCronField(field string, min, max int, fieldName string) error {
	if field == "*" {
		return nil
	}

	// Handle */N syntax
	if strings.HasPrefix(field, "*/") {
		intervalStr := strings.TrimPrefix(field, "*/")
		if interval, err := strconv.Atoi(intervalStr); err != nil || interval <= 0 {
			return fmt.Errorf("invalid %s interval: %s", fieldName, intervalStr)
		}
		return nil
	}

	// Handle exact values (including comma-separated lists)
	values := strings.Split(field, ",")
	for _, val := range values {
		val = strings.TrimSpace(val)
		if fieldValue, err := strconv.Atoi(val); err != nil {
			return fmt.Errorf("invalid %s value: %s", fieldName, val)
		} else if fieldValue < min || fieldValue > max {
			return fmt.Errorf("%s value %d out of range [%d-%d]", fieldName, fieldValue, min, max)
		}
	}

	return nil
}

// generateJobID generates a simple job ID
func generateJobID() string {
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}

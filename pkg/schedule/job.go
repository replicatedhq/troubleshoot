package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	storageDir := filepath.Join(homeDir, ".troubleshoot", "scheduled-jobs")
	os.MkdirAll(storageDir, 0755)

	return &Manager{storageDir: storageDir}
}

// CreateJob creates a new scheduled job
func (m *Manager) CreateJob(name, schedule, namespace string, auto bool, upload string) (*Job, error) {
	// Basic cron validation (just check it has 5 parts)
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid cron schedule format, expected 5 fields")
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

// generateJobID generates a simple job ID
func generateJobID() string {
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}

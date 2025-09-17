package schedule

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Daemon runs scheduled jobs
type Daemon struct {
	manager     *Manager
	running     bool
	jobMutex    sync.Mutex
	runningJobs map[string]bool // Track running jobs to prevent concurrent execution
	logger      *log.Logger
	logFile     *os.File
}

// NewDaemon creates a new daemon
func NewDaemon() (*Daemon, error) {
	manager, err := NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	// Setup persistent logging
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".troubleshoot")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	logPath := filepath.Join(logDir, "scheduler.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	logger := log.New(logFile, "", log.LstdFlags)

	return &Daemon{
		manager:     manager,
		running:     false,
		runningJobs: make(map[string]bool),
		logger:      logger,
		logFile:     logFile,
	}, nil
}

// Start starts the daemon to monitor and execute jobs
func (d *Daemon) Start() error {
	d.running = true

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ensure signal handling is cleaned up and close log file
	defer func() {
		signal.Stop(sigChan)
		if d.logFile != nil {
			d.logFile.Close()
		}
	}()

	d.logInfo("Scheduler daemon started")
	d.logInfo("Monitoring scheduled jobs every minute...")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for d.running {
		select {
		case <-ticker.C:
			d.checkAndExecuteJobs()
		case sig := <-sigChan:
			d.logInfo(fmt.Sprintf("Received signal %v, shutting down...", sig))
			d.running = false
		}
	}

	d.logInfo("Scheduler daemon stopped")
	return nil
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	d.running = false
}

// checkAndExecuteJobs checks for jobs that should run now
func (d *Daemon) checkAndExecuteJobs() {
	jobs, err := d.manager.ListJobs()
	if err != nil {
		d.logError(fmt.Sprintf("Error loading jobs: %v", err))
		return
	}

	now := time.Now()
	for _, job := range jobs {
		if job == nil {
			continue // Skip nil jobs
		}

		if job.Enabled && d.shouldJobRun(job, now) {
			// Check if job is already running to prevent concurrent execution
			d.jobMutex.Lock()
			if d.runningJobs[job.ID] {
				d.jobMutex.Unlock()
				continue // Skip if already running
			}
			d.runningJobs[job.ID] = true
			d.jobMutex.Unlock()

			go d.executeJob(job)
		}
	}
}

// shouldJobRun checks if a job should run based on its schedule
func (d *Daemon) shouldJobRun(job *Job, now time.Time) bool {
	if job == nil {
		return false
	}

	// Prevent running multiple times in the same minute (avoid duplicates)
	// Use 90-second cooldown to ensure we don't run more than once per minute
	// even with slight timing variations in the daemon's check cycle
	if !job.LastRun.IsZero() && now.Sub(job.LastRun) < 90*time.Second {
		return false
	}

	// Parse cron schedule (minute hour day-of-month month day-of-week)
	parts := strings.Fields(job.Schedule)
	if len(parts) != 5 {
		return false
	}

	minute := parts[0]
	hour := parts[1]
	dayOfMonth := parts[2]
	month := parts[3]
	dayOfWeek := parts[4]

	// Check if current time matches all cron fields
	if !matchesCronField(minute, now.Minute()) {
		return false
	}
	if !matchesCronField(hour, now.Hour()) {
		return false
	}
	if !matchesCronField(dayOfMonth, now.Day()) {
		return false
	}
	if !matchesCronField(month, int(now.Month())) {
		return false
	}
	// Day of week: Sunday = 0, Monday = 1, etc.
	if !matchesCronField(dayOfWeek, int(now.Weekday())) {
		return false
	}

	return true
}

// matchesCronField checks if a cron field matches the current time value
func matchesCronField(field string, currentValue int) bool {
	if field == "*" {
		return true
	}

	// Handle */N syntax (e.g., */2 for every 2 minutes)
	if strings.HasPrefix(field, "*/") {
		intervalStr := strings.TrimPrefix(field, "*/")
		if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
			return currentValue%interval == 0
		}
		return false // Invalid interval format
	}

	// Handle comma-separated lists (e.g., "1,15,30")
	values := strings.Split(field, ",")
	for _, val := range values {
		val = strings.TrimSpace(val)
		if fieldValue, err := strconv.Atoi(val); err == nil {
			if currentValue == fieldValue {
				return true
			}
		}
	}

	return false
}

// findSupportBundleBinary finds the support-bundle binary path
func findSupportBundleBinary() (string, error) {
	// First try current directory
	if _, err := os.Stat("./support-bundle"); err == nil {
		abs, _ := filepath.Abs("./support-bundle")
		return abs, nil
	}

	// Try relative to current binary location
	if execPath, err := os.Executable(); err == nil {
		supportBundlePath := filepath.Join(filepath.Dir(execPath), "support-bundle")
		if _, err := os.Stat(supportBundlePath); err == nil {
			return supportBundlePath, nil
		}
	}

	// Try PATH
	if path, err := exec.LookPath("support-bundle"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("support-bundle binary not found")
}

// executeJob runs a support bundle collection
func (d *Daemon) executeJob(job *Job) {
	if job == nil {
		return
	}

	// Ensure we mark the job as not running when done
	defer func() {
		d.jobMutex.Lock()
		delete(d.runningJobs, job.ID)
		d.jobMutex.Unlock()
	}()

	d.logInfo(fmt.Sprintf("Executing job: %s", job.Name))

	// Build command arguments (no subcommand needed - binary IS support-bundle)
	args := []string{}
	if job.Namespace != "" {
		args = append(args, "--namespace", job.Namespace)
	}
	if job.Auto {
		args = append(args, "--auto")
	}
	if job.Upload != "" {
		args = append(args, "--auto-upload")
		// Add license and app flags if available in the future
		// if job.LicenseID != "" {
		//     args = append(args, "--license-id", job.LicenseID)
		// }
		// if job.AppSlug != "" {
		//     args = append(args, "--app-slug", job.AppSlug)
		// }
	}

	// Disable auto-update for scheduled jobs
	args = append(args, "--auto-update=false")

	// Find support-bundle binary
	supportBundleBinary, err := findSupportBundleBinary()
	if err != nil {
		d.logError(fmt.Sprintf("Job failed: %s - cannot find support-bundle binary: %v", job.Name, err))
		return
	}

	// Execute support-bundle command directly with output capture
	cmd := exec.Command(supportBundleBinary, args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	if err != nil {
		d.logError(fmt.Sprintf("Job failed: %s - %v", job.Name, err))
		if len(output) > 0 {
			d.logError(fmt.Sprintf("Command output for %s:\n%s", job.Name, string(output)))
		}
		return
	}

	d.logInfo(fmt.Sprintf("Job completed: %s", job.Name))

	// Log key information but skip verbose JSON output
	if len(output) > 0 {
		outputStr := string(output)

		// Extract and log only the important parts
		if strings.Contains(outputStr, "Successfully uploaded support bundle") {
			d.logInfo(fmt.Sprintf("Upload successful for job: %s", job.Name))
		}
		if strings.Contains(outputStr, "Auto-upload failed:") {
			// Log upload failures in detail
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Auto-upload failed:") {
					d.logError(fmt.Sprintf("Upload failed for job %s: %s", job.Name, strings.TrimSpace(line)))
				}
			}
		}
		if strings.Contains(outputStr, "archivePath") {
			// Extract just the archive name
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				if strings.Contains(line, "archivePath") {
					d.logInfo(fmt.Sprintf("Archive created for job %s: %s", job.Name, strings.TrimSpace(line)))
					break
				}
			}
		}
	}

	// Update job stats only on success
	job.RunCount++
	job.LastRun = time.Now()
	if err := d.manager.saveJob(job); err != nil {
		d.logError(fmt.Sprintf("Warning: Failed to save job statistics for %s: %v", job.Name, err))
	}
}

// logInfo logs an info message to both console and file
func (d *Daemon) logInfo(message string) {
	fmt.Printf("✓ %s\n", message)
	if d.logger != nil {
		d.logger.Printf("INFO: %s", message)
	}
}

// logError logs an error message to both console and file
func (d *Daemon) logError(message string) {
	fmt.Printf("❌ %s\n", message)
	if d.logger != nil {
		d.logger.Printf("ERROR: %s", message)
	}
}

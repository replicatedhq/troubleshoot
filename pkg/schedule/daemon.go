package schedule

import (
	"fmt"
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
}

// NewDaemon creates a new daemon
func NewDaemon() (*Daemon, error) {
	manager, err := NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	return &Daemon{
		manager:     manager,
		running:     false,
		runningJobs: make(map[string]bool),
	}, nil
}

// Start starts the daemon to monitor and execute jobs
func (d *Daemon) Start() error {
	d.running = true

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ensure signal handling is cleaned up
	defer signal.Stop(sigChan)

	fmt.Println("✓ Scheduler daemon started")
	fmt.Println("Monitoring scheduled jobs every minute...")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for d.running {
		select {
		case <-ticker.C:
			d.checkAndExecuteJobs()
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			d.running = false
		}
	}

	fmt.Println("✓ Scheduler daemon stopped")
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
		fmt.Printf("Error loading jobs: %v\n", err)
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

	fmt.Printf("🔄 Executing job: %s\n", job.Name)

	// Build command arguments (no subcommand needed - binary IS support-bundle)
	args := []string{}
	if job.Namespace != "" {
		args = append(args, "--namespace", job.Namespace)
	}
	if job.Auto {
		args = append(args, "--auto")
	}
	if job.Upload != "" {
		args = append(args, "--upload", job.Upload)
	}

	// Find support-bundle binary
	supportBundleBinary, err := findSupportBundleBinary()
	if err != nil {
		fmt.Printf("❌ Job failed: %s - cannot find support-bundle binary: %v\n", job.Name, err)
		return
	}

	// Execute support-bundle command directly
	cmd := exec.Command(supportBundleBinary, args...)
	err = cmd.Run()

	if err != nil {
		fmt.Printf("❌ Job failed: %s - %v\n", job.Name, err)
		return
	}

	fmt.Printf("✅ Job completed: %s\n", job.Name)

	// Update job stats only on success
	job.RunCount++
	job.LastRun = time.Now()
	if err := d.manager.saveJob(job); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save job statistics for %s: %v\n", job.Name, err)
	}
}

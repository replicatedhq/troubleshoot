package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Daemon runs scheduled jobs
type Daemon struct {
	manager *Manager
	running bool
}

// NewDaemon creates a new daemon
func NewDaemon() *Daemon {
	return &Daemon{
		manager: NewManager(),
		running: false,
	}
}

// Start starts the daemon to monitor and execute jobs
func (d *Daemon) Start() error {
	d.running = true

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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
		if job.Enabled && d.shouldJobRun(job, now) {
			go d.executeJob(job)
		}
	}
}

// shouldJobRun checks if a job should run based on its schedule
func (d *Daemon) shouldJobRun(job *Job, now time.Time) bool {
	// Prevent running in the same minute (avoid duplicates)
	if !job.LastRun.IsZero() && now.Sub(job.LastRun) < 30*time.Second {
		return false
	}

	// Parse cron schedule (simplified)
	parts := strings.Fields(job.Schedule)
	if len(parts) != 5 {
		return false
	}

	minute := parts[0]
	hour := parts[1]

	// Check if current time matches schedule (with */N support)
	if !matchesCronField(minute, now.Minute()) {
		return false
	}
	if !matchesCronField(hour, now.Hour()) {
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
	}

	// Handle exact matches
	if fieldValue, err := strconv.Atoi(field); err == nil {
		return currentValue == fieldValue
	}

	return false
}

// findTroubleshootBinary finds the troubleshoot binary path
func findTroubleshootBinary() (string, error) {
	// First try current directory
	if _, err := os.Stat("./troubleshoot"); err == nil {
		abs, _ := filepath.Abs("./troubleshoot")
		return abs, nil
	}

	// Try relative to current binary location
	if execPath, err := os.Executable(); err == nil {
		troubleshootPath := filepath.Join(filepath.Dir(execPath), "troubleshoot")
		if _, err := os.Stat(troubleshootPath); err == nil {
			return troubleshootPath, nil
		}
	}

	// Try PATH
	if path, err := exec.LookPath("troubleshoot"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("troubleshoot binary not found")
}

// executeJob runs a support bundle collection
func (d *Daemon) executeJob(job *Job) {
	fmt.Printf("🔄 Executing job: %s\n", job.Name)

	// Build command arguments
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

	// Find troubleshoot binary
	troubleshootBinary, err := findTroubleshootBinary()
	if err != nil {
		fmt.Printf("❌ Job failed: %s - cannot find troubleshoot binary: %v\n", job.Name, err)
		return
	}

	// Execute troubleshoot binary directly (it IS the support-bundle command)
	cmd := exec.Command(troubleshootBinary, args...)
	err = cmd.Run()

	// Update job stats
	job.RunCount++
	job.LastRun = time.Now()
	d.manager.saveJob(job)

	if err != nil {
		fmt.Printf("❌ Job failed: %s - %v\n", job.Name, err)
	} else {
		fmt.Printf("✅ Job completed: %s\n", job.Name)
	}
}

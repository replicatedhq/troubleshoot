package schedule

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestManager_CreateJob(t *testing.T) {
	// Use temporary directory for testing
	tempDir, err := os.MkdirTemp("", "schedule-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &Manager{storageDir: tempDir}

	// Test job creation
	job, err := manager.CreateJob("test-job", "0 2 * * *", "default", true, "s3://bucket")
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if job.Name != "test-job" {
		t.Errorf("Job name = %s, want test-job", job.Name)
	}

	if job.Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %s, want 0 2 * * *", job.Schedule)
	}

	if !job.Enabled {
		t.Error("Job should be enabled by default")
	}
}

func TestManager_ListJobs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "schedule-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &Manager{storageDir: tempDir}

	// Create test jobs
	_, err = manager.CreateJob("job1", "0 1 * * *", "ns1", false, "")
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	_, err = manager.CreateJob("job2", "0 2 * * *", "ns2", true, "s3://bucket")
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// List jobs
	jobs, err := manager.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestManager_DeleteJob(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "schedule-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &Manager{storageDir: tempDir}

	// Create and delete job
	job, err := manager.CreateJob("temp-job", "0 3 * * *", "default", false, "")
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	err = manager.DeleteJob(job.Name)
	if err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}

	// Verify deletion
	jobs, err := manager.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after deletion, got %d", len(jobs))
	}
}

func TestDaemon_ScheduleMatching(t *testing.T) {
	daemon := NewDaemon()

	// Test job that should run at current minute
	now := time.Now()
	job := &Job{
		Schedule: fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour()),
		LastRun:  time.Time{}, // Never run
		Enabled:  true,
	}

	if !daemon.shouldJobRun(job, now) {
		t.Error("Job should run at current time")
	}

	// Test job that just ran
	job.LastRun = now.Add(-30 * time.Second)
	if daemon.shouldJobRun(job, now) {
		t.Error("Job should not run again so soon")
	}
}

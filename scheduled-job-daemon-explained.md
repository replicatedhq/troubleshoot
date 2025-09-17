# Scheduled Jobs + Daemon: How They Work Together

## The Complete Picture

```
You create scheduled jobs → Daemon watches jobs → Jobs run automatically

┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Scheduled Job  │    │   Daemon Process │    │   Job Execution │
│                 │    │                  │    │                 │
│ Name: daily     │───▶│ ⏰ Checks time   │───▶│ ▶ Collect bundle│
│ Schedule: 2 AM  │    │ 📋 Reads jobs    │    │ ▶ Upload to S3  │
│ Task: collect   │    │ 🔄 Runs loop     │    │ ▶ Send alerts   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Step-by-Step Example

### 1. You Create a Scheduled Job (One Time Setup)
```bash
support-bundle schedule create daily-health-check \
  --cron "0 2 * * *" \
  --namespace production \
  --upload s3://my-diagnostics-bucket
```

**What this creates:**
- A job definition stored on disk
- Schedule: "Run daily at 2:00 AM"
- Task: "Collect support bundle from production namespace and upload to S3"

### 2. You Start the Daemon (One Time Setup)
```bash
support-bundle schedule daemon start
```

**What the daemon does:**
```go
// Simplified daemon logic
for {
    currentTime := time.Now()
    
    // Check all scheduled jobs
    for _, job := range scheduledJobs {
        if job.NextRunTime <= currentTime && job.Enabled {
            go runSupportBundleCollection(job)  // Run in background
            job.NextRunTime = calculateNextRun(job.Schedule)
        }
    }
    
    time.Sleep(60 * time.Second)  // Wait 1 minute, then check again
}
```

### 3. Automatic Execution (Happens Forever)
```
Day 1, 2:00 AM → Daemon sees it's time → Runs: support-bundle --namespace production
Day 2, 2:00 AM → Daemon sees it's time → Runs: support-bundle --namespace production  
Day 3, 2:00 AM → Daemon sees it's time → Runs: support-bundle --namespace production
... continues forever ...
```

## Key Benefits

### Without Scheduling (Manual)
```bash
# You have to remember to run this every day
support-bundle --namespace production
# Upload manually
# Check results manually
# Easy to forget!
```

### With Scheduling (Automatic)
```bash
# Set it up once
support-bundle schedule create daily-check --cron "0 2 * * *" --namespace production --upload s3://bucket

# Start daemon once  
support-bundle schedule daemon start

# Now it happens automatically forever:
# ✓ Collects support bundle daily at 2 AM
# ✓ Uploads to S3 automatically
# ✓ Never forgets
# ✓ You can sleep peacefully!
```

## Real-World Comparison

### Scheduled Job = Appointment in Calendar
- **Job Definition**: "Doctor appointment every 6 months"
- **Schedule**: "Next Tuesday at 3 PM"
- **Task**: "Go to doctor for checkup"

### Daemon = Personal Assistant
- **Always watching**: Checks your calendar continuously
- **Reminds you**: "It's time for your doctor appointment!"
- **Manages conflicts**: "You have 3 appointments at once, let me reschedule"
- **Never sleeps**: Works 24/7 even when you're busy

### In Troubleshoot Terms
- **Scheduled Job**: "Collect diagnostics every 6 hours from namespace 'webapp'"
- **Daemon**: Background service that watches the clock and runs collections automatically
- **Result**: Continuous monitoring without manual intervention

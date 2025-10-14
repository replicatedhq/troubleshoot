package schedule

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// CLI creates the schedule command
func CLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage scheduled support bundle jobs",
		Long: `Create and manage scheduled support bundle collection jobs.

This allows customers to schedule support bundle collection to run automatically
at specified times using standard cron syntax.`,
	}

	cmd.AddCommand(
		createCommand(),
		listCommand(),
		deleteCommand(),
		daemonCommand(),
	)

	return cmd
}

// createCommand creates the create subcommand
func createCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [job-name] --cron [schedule] [--namespace ns]",
		Short: "Create a scheduled support bundle job",
		Long: `Create a new scheduled job to automatically collect support bundles.

Examples:
  # Daily at 2 AM
  support-bundle schedule create daily-check --cron "0 2 * * *" --namespace production

  # Every 6 hours with auto-discovery and auto-upload to vendor portal
  support-bundle schedule create frequent --cron "0 */6 * * *" --namespace app --auto --upload enabled`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cronSchedule, _ := cmd.Flags().GetString("cron")
			namespace, _ := cmd.Flags().GetString("namespace")
			auto, _ := cmd.Flags().GetBool("auto")
			upload, _ := cmd.Flags().GetString("upload")
			licenseID, _ := cmd.Flags().GetString("license-id")
			appSlug, _ := cmd.Flags().GetString("app-slug")

			if cronSchedule == "" {
				return fmt.Errorf("--cron is required")
			}

			manager, err := NewManager()
			if err != nil {
				return err
			}
			job, err := manager.CreateJobWithCredentials(args[0], cronSchedule, namespace, auto, upload, licenseID, appSlug)
			if err != nil {
				return err
			}

			fmt.Printf("✓ Created scheduled job '%s' (ID: %s)\n", job.Name, job.ID)
			fmt.Printf("  Schedule: %s\n", job.Schedule)
			fmt.Printf("  Namespace: %s\n", job.Namespace)
			if auto {
				fmt.Printf("  Auto-discovery: enabled\n")
			}
			if upload != "" {
				fmt.Printf("  Auto-upload: enabled (uploads to vendor portal)\n")
			}
			if licenseID != "" {
				fmt.Printf("  License ID: %s\n", licenseID)
			}
			if appSlug != "" {
				fmt.Printf("  App Slug: %s\n", appSlug)
			}

			fmt.Printf("\n💡 To activate, start the daemon:\n")
			fmt.Printf("   support-bundle schedule daemon start\n")

			return nil
		},
	}

	cmd.Flags().StringP("cron", "c", "", "Cron expression (required)")
	cmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace (optional)")
	cmd.Flags().Bool("auto", false, "Enable auto-discovery")
	cmd.Flags().String("upload", "", "Enable auto-upload to vendor portal (any non-empty value enables auto-upload)")
	cmd.Flags().String("license-id", "", "License ID for auto-upload")
	cmd.Flags().String("app-slug", "", "Application slug for auto-upload")
	cmd.MarkFlagRequired("cron")

	return cmd
}

// listCommand creates the list subcommand
func listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all scheduled jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := NewManager()
			if err != nil {
				return err
			}
			jobs, err := manager.ListJobs()
			if err != nil {
				return err
			}

			if len(jobs) == 0 {
				fmt.Println("No scheduled jobs found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tSCHEDULE\tNAMESPACE\tAUTO\tAUTO-UPLOAD\tRUNS")

			for _, job := range jobs {
				upload := "none"
				if job.Upload != "" {
					upload = "enabled"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\t%d\n",
					job.Name, job.Schedule, job.Namespace, job.Auto, upload, job.RunCount)
			}

			return w.Flush()
		},
	}
}

// deleteCommand creates the delete subcommand
func deleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [job-name]",
		Short: "Delete a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := NewManager()
			if err != nil {
				return err
			}

			if err := manager.DeleteJob(args[0]); err != nil {
				return err
			}

			fmt.Printf("✓ Deleted job: %s\n", args[0])
			return nil
		},
	}
}

// daemonCommand creates the daemon subcommand
func daemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage scheduler daemon",
	}

	start := &cobra.Command{
		Use:   "start",
		Short: "Start the scheduler daemon",
		Long:  "Start the daemon to automatically execute scheduled jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			daemon, err := NewDaemon()
			if err != nil {
				return err
			}
			return daemon.Start()
		},
	}

	cmd.AddCommand(start)
	return cmd
}

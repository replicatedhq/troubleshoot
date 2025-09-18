package cli

import (
	"github.com/replicatedhq/troubleshoot/pkg/schedule"
	"github.com/spf13/cobra"
)

// Schedule returns the schedule command for managing scheduled support bundle jobs
func Schedule() *cobra.Command {
	return schedule.CLI()
}

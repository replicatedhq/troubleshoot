package cli

import (
	"fmt"

	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/spf13/cobra"
)

func OciFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oci-fetch [URI]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Fetch a preflight from an OCI registry and print it to standard out",
		RunE: func(cmd *cobra.Command, args []string) error {
			uri := args[0]
			data,err := oci.PullPreflightFromOCI(uri)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
	return cmd
}
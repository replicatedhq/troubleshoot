package cli

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func UploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload [bundle-file]",
		Args:  cobra.ExactArgs(1),
		Short: "Upload an existing support bundle to a vendor portal",
		Long: `Upload a support bundle archive to a vendor portal.

This command takes an existing support bundle .tar.gz file and uploads it to the specified vendor portal endpoint.

Examples:
  # Upload a bundle to production (default endpoint)
  support-bundle upload --app-id my-app support-bundle.tar.gz

  # Upload to staging with custom token
  support-bundle upload --endpoint https://api.staging.replicated.com/vendor --token my-token --app-id my-app support-bundle.tar.gz

  # Upload using environment variable for token
  TROUBLESHOOT_TOKEN=my-token support-bundle upload --app-id my-app support-bundle.tar.gz`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			bundlePath := args[0]

			// Get upload parameters
			endpoint := v.GetString("endpoint")
			token := v.GetString("token")
			appID := v.GetString("app-id")

			// Check for token in environment if not provided
			if token == "" {
				token = os.Getenv("TROUBLESHOOT_TOKEN")
			}

			// Validate required parameters
			if token == "" {
				return errors.New("--token is required (or set TROUBLESHOOT_TOKEN environment variable)")
			}
			if appID == "" {
				return errors.New("--app-id is required")
			}

			// Check if bundle file exists
			if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
				return errors.Errorf("bundle file does not exist: %s", bundlePath)
			}

			// Upload the bundle
			fmt.Fprintf(os.Stderr, "Uploading bundle %s to %s...\n", bundlePath, endpoint)
			if err := supportbundle.UploadToVandoor(bundlePath, endpoint, token, appID); err != nil {
				return errors.Wrap(err, "upload failed")
			}

			fmt.Fprintf(os.Stderr, "Bundle uploaded successfully!\n")
			return nil
		},
	}

	cmd.Flags().String("endpoint", "https://api.replicated.com/vendor", "vendor API endpoint (default: https://api.replicated.com/vendor)")
	cmd.Flags().String("token", "", "API token for authentication (or set TROUBLESHOOT_TOKEN env var)")
	cmd.Flags().String("app-id", "", "app ID to associate the bundle with")

	// endpoint defaults to production; can be overridden
	cmd.MarkFlagRequired("app-id")

	return cmd
}

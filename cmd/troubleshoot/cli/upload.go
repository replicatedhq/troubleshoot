package cli

import (
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
		Short: "Upload a support bundle to replicated.app",
		Long: `Upload a support bundle to replicated.app for analysis and troubleshooting.

This command automatically extracts the license ID from the bundle if not provided.

Examples:
  # Auto-detect license from bundle
  support-bundle upload bundle.tar.gz

  # Specify license ID explicitly
  support-bundle upload bundle.tar.gz --license-id YOUR_LICENSE_ID`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			bundlePath := args[0]

			// Check if bundle file exists
			if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
				return errors.Errorf("bundle file does not exist: %s", bundlePath)
			}

			// Get upload parameters
			licenseID := v.GetString("license-id")

			// Use auto-detection for uploads
			if err := supportbundle.UploadBundleAutoDetect(bundlePath, licenseID); err != nil {
				return errors.Wrap(err, "upload failed")
			}

			return nil
		},
	}

	cmd.Flags().String("license-id", "", "license ID for authentication (auto-detected from bundle if not provided)")

	return cmd
}

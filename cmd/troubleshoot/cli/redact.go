package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Redact() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redact [urls...]",
		Args:  cobra.MinimumNArgs(1), // TODO
		Short: "Redact information from a generated support bundle archive",
		Long: `Redaction is the process of masking sensitive information from collected data in a support bundle.
This is done using rules defined in the list of redactor manifests provided in the [urls...] command line
argument. Default built in redactors will also be run, but these would have been run when the support
bundle was generated. After redaction, the support bundle is archived once more. The resulting file will
be stored in the current directory in the path provided by the --output flag.

The [urls...] argument is a list of either oci://.., http://.., https://.. or local paths to yaml files.

For more information on redactors visit https://troubleshoot.sh/docs/redact/
		`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			// 1. Decode redactors from provided URLs
			redactors, err := supportbundle.GetRedactorsFromURIs(args)
			if err != nil {
				return err
			}

			// 2. Download the bundle and extract it
			tmpDir, bundleDir, err := analyzer.DownloadAndExtractSupportBundle(v.GetString("bundle"))
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			// 3. Represent bundle as a CollectorResult
			collectorResult, err := collect.CollectorResultFromBundle(bundleDir)
			if err != nil {
				return err
			}

			// 4. Perform redaction on the bundle
			err = collect.RedactResult(bundleDir, collectorResult, redactors, nil)
			if err != nil {
				return errors.Wrap(err, "failed to redact support bundle")
			}

			// 5. Compress the bundle once more after redacting
			output := v.GetString("output")
			if output == "" {
				output = fmt.Sprintf("redacted-support-bundle-%s.tar.gz", time.Now().Format("2006-01-02T15_04_05"))
			}
			err = collectorResult.ArchiveBundle(bundleDir, output)
			if err != nil {
				return errors.Wrap(err, "failed to create support bundle archive")
			}
			fmt.Println("Redacted support bundle:", output)
			return nil
		},
	}

	cmd.Flags().String("bundle", "", "file path of the support bundle archive to redact")
	cmd.MarkFlagRequired("bundle")
	cmd.Flags().BoolP("quiet", "q", false, "enable/disable error messaging and only show parseable output")
	cmd.Flags().StringP("output", "o", "", "file path of where to save the redacted support bundle archive (default \"redacted-support-bundle-YYYY-MM-DDTHH_MM_SS.tar.gz\")")

	return cmd
}

package cli

import (
	"fmt"

	"io/ioutil"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the current version and exit",
		Long:  `Print the current version and exit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Replicated Troubleshoot %s\n", version.Version())

			return nil
		},
	}
	return cmd
}

const VersionFilename = "version.yaml"

func writeVersionFile(path string) error {
	version := troubleshootv1beta2.SupportBundleVersion{
		ApiVersion: "troubleshoot.sh/v1beta2",
		Kind:       "SupportBundle",
		Spec: troubleshootv1beta2.SupportBundleVersionSpec{
			VersionNumber: version.Version(),
		},
	}
	b, err := yaml.Marshal(version)
	if err != nil {
		return err
	}

	filename := filepath.Join(path, VersionFilename)
	err = ioutil.WriteFile(filename, b, 0644)
	if err != nil {
		return err
	}

	return nil
}

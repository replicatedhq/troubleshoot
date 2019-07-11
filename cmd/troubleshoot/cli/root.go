package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "troubleshoot",
		Short: "Generate and manage support bundles",
		Long: `A support bundle is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a server.`,
		SilenceUsage: true,
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Run())
	cmd.AddCommand(Retrieve())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}

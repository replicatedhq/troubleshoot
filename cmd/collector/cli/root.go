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
		Use:          "collector",
		Short:        "Run the cluster-side server for bundle collection",
		Long:         ``,
		SilenceUsage: true,
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Server())

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
	viper.SetEnvPrefix("TROUBLESHOT")
	viper.AutomaticEnv()
}

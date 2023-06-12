package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var FakeDefaultSpecLables = []string{"troubleshoot.io/kind=support-bundle"}

var CollectWithoutPermissions bool
var CpuProfiling string
var Debug bool
var Interactive bool
var MemProfile string
var NoURI bool

var AditionalRedactors []string
var LoadClusterSpecs bool
var ClusterSpecLables []string

var CollectOutput string
var CollectRedact bool
var CollectSince string
var CollectSinceTime string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "troubleshoot",
	Short: "A brief description of your application",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

var collectCmd = &cobra.Command{
	Use:   "collect [spec URIs...]",
	Short: "Run collectors, redactors and analyzers, store the result",
	Run: Collect,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze [bundle path] [spec URIs]",
	Short: "Analyze an existing support bundle",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("analyzing...")
	},
}

var redactCmd = &cobra.Command{
	Use: "redact [bundle path] [spec URIs]",
	Short: "Run redactors across an existing support bundle",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("redacting with specs at: "+strings.Join(args,","))
	},
}

var inspectCmd = &cobra.Command{
	Use: "inspect [bundle path]",
	Short: "Open an interactive shell to inspect an existing support bundle with kubectl",
	Run: func(cmd *cobra.Command, args []string){
		fmt.Println("inspecting...")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// subcommands
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(redactCmd)
	rootCmd.AddCommand(inspectCmd)
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&CollectWithoutPermissions,"collect-without-permissions","",true,"always generate a support bundle, even if it some require additional permissions")
	rootCmd.PersistentFlags().StringVarP(&CpuProfiling,"cpuprofile","","","File path to write cpu profiling data")
	rootCmd.PersistentFlags().BoolVarP(&Debug,"debug","",false,"enable debug logging. This is equivalent to --v=0")
	rootCmd.PersistentFlags().BoolVarP(&Interactive,"interactive","",true,"enable/disable interactive mode")
	rootCmd.PersistentFlags().StringVarP(&MemProfile,"memprofile","","","File path to write memory profiling data")
	rootCmd.PersistentFlags().BoolVarP(&NoURI,"no-uri","",false,"When this flag is used, Troubleshoot does not attempt to retrieve the bundle referenced by the uri: field in the spec.")
	// Collect flags
	collectCmd.Flags().StringVarP(&CollectOutput,"output","o","","specify the output file path for the support bundle")
	collectCmd.Flags().BoolVarP(&CollectRedact,"redact","",true,"enable/disable default redactions")
	collectCmd.Flags().BoolVarP(&LoadClusterSpecs,"load-cluster-specs","",false,"enable/disable loading additional troubleshoot specs found within the cluster. required when no specs are provided on the command line")
	collectCmd.Flags().StringVarP(&CollectSince,"since","","","force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	collectCmd.Flags().StringVarP(&CollectSinceTime,"since-time","","","force pod logs collectors to return logs after a specific date (RFC3339)")
	collectCmd.Flags().StringArrayVarP(&ClusterSpecLables,"spec-lables","l",FakeDefaultSpecLables,"selector to filter on for loading additional support bundle specs found in secrets within the cluster")
}

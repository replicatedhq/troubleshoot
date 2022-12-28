package util

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cpuProfileFile *os.File
)

// StartProfiling starts profiling CPU and memory usage if either --cpuprofile or
// --memprofile flags were set and bound to viper configurations respectively.
func StartProfiling() error {
	v := viper.GetViper()
	if v.GetString("cpuprofile") != "" {
		var err error
		cpuProfileFile, err = os.Create(v.GetString("cpuprofile"))
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
			cpuProfileFile.Close()
			cpuProfileFile = nil
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
	}
	return nil
}

// StopProfiling stops profiling CPU and memory usage and writes the results to
// the files specified by --cpuprofile and --memprofile flags respectively.
func StopProfiling() error {
	v := viper.GetViper()

	if v.GetString("memprofile") != "" {
		f, err := os.Create(v.GetString("memprofile"))
		if err != nil {
			return fmt.Errorf("could not create memory profile: %w", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("could not write memory profile: %w", err)
		}
	}

	// Stop CPU profiling if it was started
	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		return cpuProfileFile.Close()
	}
	return nil
}

// AddProfilingFlags adds the --cpuprofile and --memprofile flags to the given command.
func AddProfilingFlags(cmd *cobra.Command) {
	// Persistent flags to make available to subcommands
	cmd.PersistentFlags().String("cpuprofile", "", "File path to write cpu profiling data")
	cmd.PersistentFlags().String("memprofile", "", "File path to write memory profiling data")
}

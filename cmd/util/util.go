package util

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func IsURL(str string) bool {
	parsed, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}

	return parsed.Scheme != ""
}

func AppName(name string) string {
	words := strings.Split(strings.Title(strings.Replace(name, "-", " ", -1)), " ")
	casedWords := []string{}
	for i, word := range words {
		if strings.ToLower(word) == "ai" {
			casedWords = append(casedWords, "AI")
		} else if strings.ToLower(word) == "io" && i > 0 {
			casedWords[i-1] += ".io"
		} else {
			casedWords = append(casedWords, word)
		}
	}

	return strings.Join(casedWords, " ")
}

// ProfiledRunE is a wrapper for cobra's RunE-type instatiation patterns that adds support
// for CPU and memory profiling. If --cpuprofile or --memprofile flags are set and bound to viper,
// "runFunc" will be profiled and the results will be written to the specified files.
func ProfiledRunE(cmd *cobra.Command, args []string, runFunc func(cmd *cobra.Command, args []string) error) error {
	v := viper.GetViper()
	if v.GetString("cpuprofile") != "" {
		f, err := os.Create(v.GetString("cpuprofile"))
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	err := runFunc(cmd, args)

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
	return err
}

// AddProfilingFlags adds the --cpuprofile and --memprofile flags to the given command.
func AddProfilingFlags(cmd *cobra.Command) {
	// Persistent flags to make available to subcommands
	cmd.PersistentFlags().String("cpuprofile", "", "write cpu profile to file")
	cmd.PersistentFlags().String("memprofile", "", "write memory profile to this file")
}

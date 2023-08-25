/*
Logging library for the troubleshoot framework.

Logging levels
TODO: Document me here => https://github.com/replicatedhq/troubleshoot/issues/1031

0: also the same as not using V() log progress related information within the framework. Logs within each component (collector/analyzers/etc) should not use this level.

1: High level logs within each component (collector/analyzers/etc) should use this level. A log such as "Ceph collector connected to the cluster" belongs here.

2: Everything else goes here. If you do not know which level to use, use this level.

The best approach is to always use V(2) then after testing your code as a whole, you can elevate the log level of the messages you find useful to V(1) or V(0).

Do not log errors in functions that return an error. Instead, return the error and let the caller log it.
*/
package logger

import (
	"flag"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

var lock sync.Mutex

// InitKlogFlags initializes klog flags and adds them to the cobra command.
func InitKlogFlags(flags *pflag.FlagSet) {
	// Initialize klog flags
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

	klogFlags.VisitAll(func(f *flag.Flag) {
		// Just the flags we want to expose in our CLI
		if f.Name == "v" {
			// If we ever want to expose the klog flags that have underscores ("_") in them
			// we need to replace them with hyphens ("-") in the flag name using
			// pflag.NormalizedName(strings.ReplaceAll(name, "_", "-")). Check how kubectl does it
			flags.AddGoFlag(f)
		}
	})
}

// InitKlog initializes klog with a specific verbosity. This is useful when we want to
// use klog in a library and we want to control the verbosity from the library's caller.
// We can use this in tests to print instrumented logs for example.
func InitKlog(verbosity int) {
	// Initialize klog flags
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

	klogFlags.VisitAll(func(f *flag.Flag) {
		// Just the flags we want to expose in our CLI
		if f.Name == "v" {
			// If we ever want to expose the klog flags that have underscores ("_") in them
			// we need to replace them with hyphens ("-") in the flag name using
			// pflag.NormalizedName(strings.ReplaceAll(name, "_", "-")). Check how kubectl does it
			f.Value.Set(fmt.Sprintf("%d", verbosity))
		}
	})
}

// SetupLogger sets up klog logger based on viper configuration.
func SetupLogger(v *viper.Viper) {
	quiet := v.GetBool("debug") || v.IsSet("v")
	SetQuiet(!quiet)
}

// SetQuiet enables or disables klog logger.
func SetQuiet(quiet bool) {
	lock.Lock()
	defer lock.Unlock()

	if quiet {
		klog.SetLogger(logr.Discard())
	} else {
		// Restore the default logger
		klog.ClearLogger()
	}
}

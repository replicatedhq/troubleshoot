package preflight

import (
	flag "github.com/spf13/pflag"
	utilpointer "k8s.io/utils/pointer"
)

const (
	flagInteractive               = "interactive"
	flagFormat                    = "format"
	flagCollectorImage            = "collector-image"
	flagCollectorPullPolicy       = "collector-pullpolicy"
	flagCollectWithoutPermissions = "collect-without-permissions"
	flagSelector                  = "selector"
	flagSinceTime                 = "since-time"
	flagSince                     = "since"
	flagOutput                    = "output"
	flagDebug                     = "debug"
)

type PreflightFlags struct {
	Interactive               *bool
	Format                    *string
	CollectorImage            *string
	CollectorPullPolicy       *string
	CollectWithoutPermissions *bool
	Selector                  *string
	SinceTime                 *string
	Since                     *string
	Output                    *string
	Debug                     *bool
}

var preflightFlags *PreflightFlags

func NewPreflightFlags() *PreflightFlags {
	return &PreflightFlags{
		Interactive:               utilpointer.Bool(true),
		Format:                    utilpointer.String("human"),
		CollectorImage:            utilpointer.String(""),
		CollectorPullPolicy:       utilpointer.String(""),
		CollectWithoutPermissions: utilpointer.Bool(true),
		Selector:                  utilpointer.String(""),
		SinceTime:                 utilpointer.String(""),
		Since:                     utilpointer.String(""),
		Output:                    utilpointer.String("o"),
		Debug:                     utilpointer.Bool(false),
	}
}

func AddFlags(flags *flag.FlagSet) {
	if preflightFlags == nil {
		preflightFlags = NewPreflightFlags()
	}

	preflightFlags.addFlags(flags)
}

// Reset flags for preflightFlags
func ResetFlags() {
	if preflightFlags != nil {
		preflightFlags = NewPreflightFlags()
	}
}

// AddFlags binds client configuration flags to a given flagset
func (f *PreflightFlags) addFlags(flags *flag.FlagSet) {
	if preflightFlags == nil {
		preflightFlags = NewPreflightFlags()
	}

	if f.Interactive != nil {
		flags.BoolVar(f.Interactive, flagInteractive, *f.Interactive, "interactive preflights")
	}
	if f.Format != nil {
		flags.StringVar(f.Format, flagFormat, *f.Format, "output format, one of human, json, yaml. only used when interactive is set to false")
	}

	if f.CollectorImage != nil {
		flags.StringVar(f.CollectorImage, flagCollectorImage, *f.CollectorImage, "the full name of the collector image to use")
	}
	if f.CollectorPullPolicy != nil {
		flags.StringVar(f.CollectorPullPolicy, flagCollectorPullPolicy, *f.CollectorPullPolicy, "the pull policy of the collector image")
	}
	if f.CollectWithoutPermissions != nil {
		flags.BoolVar(f.CollectWithoutPermissions, flagCollectWithoutPermissions, *f.CollectWithoutPermissions, "always run preflight checks even if some require permissions that preflight does not have")
	}
	if f.Selector != nil {
		flags.StringVar(f.Selector, flagSelector, *f.Selector, "selector (label query) to filter remote collection nodes on.")
	}
	if f.SinceTime != nil {
		flags.StringVar(f.SinceTime, flagSinceTime, *f.SinceTime, "force pod logs collectors to return logs after a specific date (RFC3339)")
	}
	if f.Since != nil {
		flags.StringVar(f.Since, flagSince, *f.Since, "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	}
	if f.Output != nil {
		flags.StringVarP(f.Output, flagOutput, *f.Output, "", "specify the output file path for the preflight checks")
	}
	if f.Debug != nil {
		flags.BoolVar(f.Debug, flagDebug, *f.Debug, "enable debug logging")
	}
}

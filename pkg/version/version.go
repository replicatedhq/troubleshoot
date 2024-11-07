package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
)

var (
	build Build
)

// Build holds details about this build of the binary
type Build struct {
	Version      string     `json:"version,omitempty"`
	GitSHA       string     `json:"git,omitempty"`
	BuildTime    time.Time  `json:"buildTime,omitempty"`
	TimeFallback string     `json:"buildTimeFallback,omitempty"`
	GoInfo       GoInfo     `json:"go,omitempty"`
	RunAt        *time.Time `json:"runAt,omitempty"`
}

type GoInfo struct {
	Version  string `json:"version,omitempty"`
	Compiler string `json:"compiler,omitempty"`
	OS       string `json:"os,omitempty"`
	Arch     string `json:"arch,omitempty"`
}

// initBuild sets up the version info from build args or imported modules in go.mod
func initBuild() {
	// TODO: Can we get the module name at runtime somehow?
	tsModuleName := "github.com/replicatedhq/troubleshoot"

	if version == "" {
		// Lets attempt to get the version from runtime build info
		// We will go through all the dependencies to find the
		// troubleshoot module version. Its OK if we cannot read
		// the buildinfo, we just won't have a version set
		bi, ok := debug.ReadBuildInfo()
		if ok {
			for _, dep := range bi.Deps {
				if dep.Path == tsModuleName {
					version = dep.Version
					break
				}
			}
		}
	}

	build.Version = version
	if len(gitSHA) >= 7 {
		build.GitSHA = gitSHA[:7]
	}

	var err error
	build.BuildTime, err = time.Parse(time.RFC3339, buildTime)
	if err != nil {
		build.TimeFallback = buildTime
	}

	build.GoInfo = getGoInfo()
	build.RunAt = &RunAt
}

// GetBuild gets the build
func GetBuild() Build {
	return build
}

// Version gets the version
func Version() string {
	return build.Version
}

// GitSHA gets the gitsha
func GitSHA() string {
	return build.GitSHA
}

// BuildTime gets the build time
func BuildTime() time.Time {
	return build.BuildTime
}

func getGoInfo() GoInfo {
	return GoInfo{
		Version:  runtime.Version(),
		Compiler: runtime.Compiler,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
}

func GetUserAgent() string {
	return fmt.Sprintf("Replicated_Troubleshoot/%s", Version())
}

func GetVersionFile() (string, error) {
	// TODO: Should this type be agnostic to the tool?
	// i.e should it be a TroubleshootVersion instead?
	version := troubleshootv1beta2.SupportBundleVersion{
		ApiVersion: "troubleshoot.sh/v1beta2",
		Kind:       "SupportBundle",
		Spec: troubleshootv1beta2.SupportBundleVersionSpec{
			VersionNumber: Version(),
		},
	}
	b, err := yaml.Marshal(version)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal version data")
	}

	return string(b), nil
}

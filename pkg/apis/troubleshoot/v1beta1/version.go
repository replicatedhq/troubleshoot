package v1beta1

import (
	"github.com/replicatedhq/troubleshoot/pkg/version"
)

type SupportBundleVersion struct {
	ApiVersion  string        `json:"apiVersion" yaml:"apiVersion"`
	Kind        string        `json:"kind" yaml:"kind"`
	VersionInfo version.Build `json:"versionInfo" yaml:"versionInfo"`
}

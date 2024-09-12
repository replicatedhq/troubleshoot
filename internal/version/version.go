package version

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	libVersion "github.com/replicatedhq/troubleshoot/pkg/version"
	"gopkg.in/yaml.v2"
)

func GetVersionFile() (io.Reader, error) {
	// TODO: Should this type be agnostic to the tool?
	// i.e should it be a TroubleshootVersion instead?
	version := troubleshootv1beta2.SupportBundleVersion{
		ApiVersion: "troubleshoot.sh/v1beta2",
		Kind:       "SupportBundle",
		Spec: troubleshootv1beta2.SupportBundleVersionSpec{
			VersionNumber: libVersion.Version(),
		},
	}
	b, err := yaml.Marshal(version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal version data")
	}

	return bytes.NewBuffer(b), nil
}

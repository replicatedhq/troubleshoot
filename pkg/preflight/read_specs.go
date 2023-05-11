package preflight

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/scheme"
)

type documentHead struct {
	Kind     string           `yaml:"kind"`
	Metadata documentMetadata `yaml:"metadata",omitempty`
}

type documentMetadata struct {
	Labels documentMetadataLabels `yaml:"labels",omitempty`
}

type documentMetadataLabels struct {
	TroubleshootKind string `yaml:"troubleshoot.io/kind",omitempty`
}

type PreflightSpecs struct {
	PreflightSpec     *troubleshootv1beta2.Preflight
	HostPreflightSpec *troubleshootv1beta2.HostPreflight
	UploadResultSpecs []*troubleshootv1beta2.Preflight
}

func (p *PreflightSpecs) Read(args []string) error {
	var preflightContent []byte
	var preflightSpec *troubleshootv1beta2.Preflight
	var hostPreflightSpec *troubleshootv1beta2.HostPreflight
	var uploadResultSpecs []*troubleshootv1beta2.Preflight
	var err error

	for _, v := range args {
		if strings.HasPrefix(v, "secret/") {
			// format secret/namespace-name/secret-name
			pathParts := strings.Split(v, "/")
			if len(pathParts) != 3 {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("path %s must have 3 components", v))
			}

			spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "preflight-spec")
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to get spec from secret"))
			}

			preflightContent = spec
		} else if _, err = os.Stat(v); err == nil {
			b, err := os.ReadFile(v)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
			}

			preflightContent = b
		} else if v == "-" {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
			}
			preflightContent = b
		} else {
			u, err := url.Parse(v)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
			}

			if u.Scheme == "oci" {
				content, err := oci.PullPreflightFromOCI(v)
				if err != nil {
					if err == oci.ErrNoRelease {
						return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", v))
					}

					return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				preflightContent = content
			} else {
				if !util.IsURL(v) {
					return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, fmt.Errorf("%s is not a URL and was not found (err %s)", v, err))
				}

				req, err := http.NewRequest("GET", v, nil)
				if err != nil {
					// exit code: should this be catch all or spec issues...?
					return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
				}
				req.Header.Set("User-Agent", "Replicated_Preflight/v1beta2")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					// exit code: should this be catch all or spec issues...?
					return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				preflightContent = body
			}
		}

		multidocs := strings.Split(string(preflightContent), "\n---\n")

		for _, doc := range multidocs {
			var parsedDocHead documentHead

			err := yaml.Unmarshal([]byte(doc), &parsedDocHead)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to parse yaml"))
			}

			// We're going to look for either of "kind: Preflight" OR "kind: Secret" with the label "troubleshoot.io/kind: preflight"

			if parsedDocHead.Kind != "Preflight" && parsedDocHead.Kind != "Secret" {
				continue
			}

			if parsedDocHead.Kind == "Secret" {
				if parsedDocHead.Metadata.Labels.TroubleshootKind == "preflight" {
					// In a Secret, we need to get the document out of the data.`preflight.yaml` or stringData.`preflight.yaml` (stringData takes precedence)
					// TODO: implement
				} else {
					// Not a preflight spec, skip
					continue
				}
			}

			preflightContent, err = docrewrite.ConvertToV1Beta2([]byte(doc))
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to convert to v1beta2"))
			}

			troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
			decode := scheme.Codecs.UniversalDeserializer().Decode
			obj, _, err := decode([]byte(preflightContent), nil, nil)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrapf(err, "failed to parse %s", v))
			}

			if spec, ok := obj.(*troubleshootv1beta2.Preflight); ok {
				if spec.Spec.UploadResultsTo == "" {
					preflightSpec = ConcatPreflightSpec(preflightSpec, spec)
				} else {
					uploadResultSpecs = append(uploadResultSpecs, spec)
				}
			} else if spec, ok := obj.(*troubleshootv1beta2.HostPreflight); ok {
				hostPreflightSpec = ConcatHostPreflightSpec(hostPreflightSpec, spec)
			}
		}
	}

	return nil
}

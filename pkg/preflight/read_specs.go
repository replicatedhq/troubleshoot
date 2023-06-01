package preflight

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/types"
)

type PreflightSpecs struct {
	PreflightSpec     *troubleshootv1beta2.Preflight
	HostPreflightSpec *troubleshootv1beta2.HostPreflight
	UploadResultSpecs []*troubleshootv1beta2.Preflight
}

func (p *PreflightSpecs) Read(args []string) error {
	var preflightContent []byte
	var err error

	// TODO: Earmarked for cleanup in favour of loader.LoadFromArgs(args []string)
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

		kinds, err := loader.LoadFromBytes(preflightContent)
		if err != nil {
			return err
		}

		for _, v := range kinds.Preflights {
			if v.Spec.UploadResultsTo == "" {
				p.PreflightSpec = ConcatPreflightSpec(p.PreflightSpec, &v)
			} else {
				p.UploadResultSpecs = append(p.UploadResultSpecs, &v)
			}
		}

		for _, v := range kinds.HostPreflights {
			p.HostPreflightSpec = ConcatHostPreflightSpec(p.HostPreflightSpec, &v)
		}
	}

	return nil
}

package util

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/types"
)

// RawSpecsFromArgs returns a slice of specs from the given args.
// The args can be a file path, a url, or a secret path.
// TODO: Load specs from discovered spec in a k8s cluster
func RawSpecsFromArgs(args []string) ([]string, error) {
	// We can use strings instead of bytes here because the specs are
	// not meant to be modified, only read. Strings are easier to work with.
	rawSpecs := []string{}

	for _, v := range args {
		if strings.HasPrefix(v, "secret/") {
			// format secret/namespace-name/secret-name
			pathParts := strings.Split(v, "/")
			if len(pathParts) != 3 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("path %s must have 3 components", v))
			}

			spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "preflight-spec")
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to get spec from secret"))
			}

			rawSpecs = append(rawSpecs, string(spec))
		} else if _, err := os.Stat(v); err == nil {
			b, err := os.ReadFile(v)
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
			}

			rawSpecs = append(rawSpecs, string(b))
		} else if v == "-" {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
			}
			rawSpecs = append(rawSpecs, string(b))
		} else {
			u, err := url.Parse(v)
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
			}

			if u.Scheme == "oci" {
				content, err := oci.PullPreflightFromOCI(v)
				if err != nil {
					if err == oci.ErrNoRelease {
						return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", v))
					}

					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				rawSpecs = append(rawSpecs, string(content))
			} else {
				if !IsURL(v) {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, fmt.Errorf("%s is not a URL and was not found (err %s)", v, err))
				}

				req, err := http.NewRequest("GET", v, nil)
				if err != nil {
					// exit code: should this be catch all or spec issues...?
					return nil, types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
				}
				req.Header.Set("User-Agent", "Replicated_Preflight/v1beta2")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					// exit code: should this be catch all or spec issues...?
					return nil, types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				rawSpecs = append(rawSpecs, string(body))
			}
		}
	}

	return rawSpecs, nil
}

package specs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/spf13/viper"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// SplitTroubleshootSecretLabelSelector splits a label selector into two selectors, if applicable:
// 1. troubleshoot.io/kind=support-bundle and non-troubleshoot (if contains) labels selector.
// 2. troubleshoot.sh/kind=support-bundle and non-troubleshoot (if contains) labels selector.
func SplitTroubleshootSecretLabelSelector(ctx context.Context, labelSelector labels.Selector) ([]string, error) {

	klog.V(1).Infof("Split %q selector into troubleshoot and non-troubleshoot labels selector separately, if applicable", labelSelector.String())

	selectorRequirements, selectorSelectable := labelSelector.Requirements()
	if !selectorSelectable {
		return nil, errors.Errorf("Selector %q is not selectable", labelSelector.String())
	}

	var troubleshootReqs, otherReqs []labels.Requirement

	for _, req := range selectorRequirements {
		if req.Key() == constants.TroubleshootIOLabelKey || req.Key() == constants.TroubleshootSHLabelKey {
			troubleshootReqs = append(troubleshootReqs, req)
		} else {
			otherReqs = append(otherReqs, req)
		}
	}

	parsedSelectorStrings := make([]string, 0)
	// Combine each troubleshoot requirement with other requirements to form new selectors
	s := labelSelector.String()
	if len(troubleshootReqs) == 0 && s != "" {
		return []string{s}, nil
	}

	for _, tReq := range troubleshootReqs {
		reqs := append(otherReqs, tReq)
		newSelector := labels.NewSelector().Add(reqs...)
		parsedSelectorStrings = append(parsedSelectorStrings, newSelector.String())
	}

	return parsedSelectorStrings, nil
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// LoadFromCLIArgs loads troubleshoot specs from args passed to a CLI command.
// This loader function is meant for troubleshoot CLI commands only, hence not making it public.
// It will contain opinionated logic for CLI commands such as interpreting viper flags,
// supporting secret/ uri format, downloading from OCI registry and other URLs, etc.
func LoadFromCLIArgs(ctx context.Context, client kubernetes.Interface, args []string, vp *viper.Viper) (*loader.TroubleshootKinds, error) {
	// Let's always ensure we have a context
	if ctx == nil {
		ctx = context.Background()
	}

	rawSpecs := []string{}

	for _, v := range args {
		if strings.HasPrefix(v, "secret/") {
			// format secret/namespace-name/secret-name
			pathParts := strings.Split(v, "/")
			if len(pathParts) != 3 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("secret path %q must have 3 components", v))
			}

			data, err := LoadFromSecret(ctx, client, pathParts[1], pathParts[2])
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to get spec from secret"))
			}

			// Append all data in the secret. Some may not be specs, but that's ok. They will be ignored.
			for _, spec := range data {
				rawSpecs = append(rawSpecs, string(spec))
			}
		} else if strings.HasPrefix(v, "configmap/") {
			// format configmap/namespace-name/configmap-name
			pathParts := strings.Split(v, "/")
			if len(pathParts) != 3 {
				return nil, errors.Errorf("configmap path %q must have 3 components", v)
			}

			data, err := LoadFromConfigMap(ctx, client, pathParts[1], pathParts[2])
			if err != nil {
				return nil, errors.Wrap(err, "failed to get spec from configmap")
			}

			// Append all data in the configmap. Some may not be specs, but that's ok. They will be ignored.
			for _, spec := range data {
				rawSpecs = append(rawSpecs, spec)
			}
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
				// TODO: We need to also pull support-bundle images from OCI
				content, err := oci.PullPreflightFromOCI(v)
				if err != nil {
					if err == oci.ErrNoRelease {
						return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", v))
					}

					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				rawSpecs = append(rawSpecs, string(content))
			} else {
				if !util.IsURL(v) {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, fmt.Errorf("%s is not a URL and was not found (err %s)", v, err))
				}

				req, err := http.NewRequestWithContext(ctx, "GET", v, nil)
				if err != nil {
					// exit code: should this be catch all or spec issues...?
					return nil, types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
				}
				req.Header.Set("User-Agent", "Replicated_Preflight/v1beta2")
				resp, err := httpClient.Do(req)
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

	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs:              rawSpecs,
		IgnoreUpdateDownloads: vp.GetBool("no-uri"),
	})
	if err != nil {
		return nil, err
	}

	if vp.GetBool("load-cluster-specs") {
		clusterKinds, err := LoadFromCluster(ctx, client, vp.GetStringSlice("selector"), vp.GetString("namespace"))
		if err != nil {
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
		}

		kinds.Add(clusterKinds)
	}

	return kinds, nil
}

// LoadFromCluster loads troubleshoot specs from the cluster based on the provided labels.
// By default this will be troubleshoot.io/kind=support-bundle and troubleshoot.sh/kind=support-bundle
// labels. We search for secrets and configmaps with the label selector and extract the raw data.
// We then load the specs from the raw data. If the user does not have sufficient permissions
// to list & read secrets and configmaps from all namespaces, we will fallback to trying each
// namespace individually, and eventually default to the configured kubeconfig namespace.
func LoadFromCluster(ctx context.Context, client kubernetes.Interface, selectors []string, ns string) (*loader.TroubleshootKinds, error) {
	if reflect.DeepEqual(selectors, []string{"troubleshoot.sh/kind=support-bundle"}) {
		// Its the default selector so we append troubleshoot.io/kind=support-bundle to it due to backwards compatibility
		selectors = append(selectors, "troubleshoot.io/kind=support-bundle")
	}

	labelSelector := strings.Join(selectors, ",")

	parsedSelector, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse selector")
	}

	// List of namespaces we want to search for secrets and configmaps with support bundle specs
	namespaces := []string{}
	if ns != "" {
		// Just progress with the namespace provided
		namespaces = []string{ns}
	} else {
		// Check if I can read secrets and configmaps in all namespaces
		ican, err := k8sutil.CanIListAndGetAllSecretsAndConfigMaps(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if I can read secrets and configmaps")
		}
		klog.V(1).Infof("Can I read any secrets and configmaps: %v", ican)

		if ican {
			// I can read secrets and configmaps in all namespaces
			// No need to iterate over all namespaces
			namespaces = []string{""}
		} else {
			// Get list of all namespaces and try to find specs from each namespace
			nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				if k8serrors.IsForbidden(err) {
					kubeconfig := k8sutil.GetKubeconfig()
					ns, _, err := kubeconfig.Namespace()
					if err != nil {
						return nil, errors.Wrap(err, "failed to get namespace from kubeconfig")
					}
					// If we are not allowed to list namespaces, just use the default namespace
					// configured in the kubeconfig
					namespaces = []string{ns}
				} else {
					return nil, errors.Wrap(err, "failed to list namespaces")
				}
			}

			for _, ns := range nsList.Items {
				namespaces = append(namespaces, ns.Name)
			}
		}
	}

	var rawSpecs []string

	parsedSelectorStrings, err := SplitTroubleshootSecretLabelSelector(ctx, parsedSelector)
	if err != nil {
		klog.Errorf("failed to parse troubleshoot labels selector %s", err)
	}

	// Iteratively search for troubleshoot specs in all namespaces using the given selectors
	for _, parsedSelectorString := range parsedSelectorStrings {
		klog.V(1).Infof("Search specs from [%q] namespace using %q selector", strings.Join(namespaces, ", "), parsedSelectorString)
		for _, ns := range namespaces {
			for _, key := range []string{constants.SupportBundleKey, constants.PreflightKey, constants.RedactorKey} {
				specs, err := LoadFromSecretMatchingLabel(ctx, client, parsedSelectorString, ns, key)
				if err != nil {
					if !k8serrors.IsForbidden(err) {
						klog.Errorf("failed to load support bundle spec from secrets: %s", err)
					} else {
						klog.Warningf("Reading secrets from %q namespace forbidden", ns)
					}
				}
				rawSpecs = append(rawSpecs, specs...)

				specs, err = LoadFromConfigMapMatchingLabel(ctx, client, parsedSelectorString, ns, key)
				if err != nil {
					if !k8serrors.IsForbidden(err) {
						klog.Errorf("failed to load support bundle spec from configmap: %s", err)
					} else {
						klog.Warningf("Reading configmaps from %q namespace forbidden", ns)
					}
				}
				rawSpecs = append(rawSpecs, specs...)
			}
		}
	}

	// Load troubleshoot specs from the raw specs
	return loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: rawSpecs,
	})
}

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

	var kindsFromURL *loader.TroubleshootKinds
	rawSpecs := []string{}

	for _, v := range args {
		if strings.HasPrefix(v, "secret/") {
			// format secret/namespace-name/secret-name[/data-key]
			pathParts := strings.Split(v, "/")
			if len(pathParts) > 4 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("secret path %s must have at most 4 components", v))
			}
			if len(pathParts) < 3 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("secret path %s must have at least 3 components", v))
			}

			data, err := LoadFromSecret(ctx, client, pathParts[1], pathParts[2])
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to get spec from secret"))
			}

			// If we have a key defined, then load specs from that key only.
			if len(pathParts) == 4 {
				spec, ok := data[pathParts[3]]
				if ok {
					rawSpecs = append(rawSpecs, string(spec))
				}
			} else {
				// Append all data in the secret. Some may not be specs, but that's ok. They will be ignored.
				for _, spec := range data {
					rawSpecs = append(rawSpecs, string(spec))
				}
			}
		} else if strings.HasPrefix(v, "configmap/") {
			// format configmap/namespace-name/configmap-name[/data-key]
			pathParts := strings.Split(v, "/")
			if len(pathParts) > 4 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("configmap path %s must have at most 4 components", v))
			}
			if len(pathParts) < 3 {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("configmap path %s must have at least 3 components", v))
			}

			data, err := LoadFromConfigMap(ctx, client, pathParts[1], pathParts[2])
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to get spec from configmap"))
			}

			// If we have a key defined, then load specs from that key only.
			if len(pathParts) == 4 {
				spec, ok := data[pathParts[3]]
				if ok {
					rawSpecs = append(rawSpecs, spec)
				}
			} else {
				// Append all data in the configmap. Some may not be specs, but that's ok. They will be ignored.
				for _, spec := range data {
					rawSpecs = append(rawSpecs, spec)
				}
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
				content, err := oci.PullSpecsFromOCI(ctx, v)
				if err != nil {
					if err == oci.ErrNoRelease {
						return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", v))
					}

					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				rawSpecs = append(rawSpecs, content...)
			} else {
				if !util.IsURL(v) {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, fmt.Errorf("%s is not a URL and was not found", v))
				}

				parsedURL, err := url.ParseRequestURI(v)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}

				var specFromURL string
				if parsedURL.Host == "kots.io" {
					// To download specs from kots.io, we need to set the User-Agent header
					specFromURL, err = downloadFromHttpURL(ctx, v, map[string]string{
						"User-Agent": "Replicated_Troubleshoot/v1beta1",
					})
					if err != nil {
						return nil, err
					}
				} else {
					specFromURL, err = downloadFromHttpURL(ctx, v, nil)
					if err != nil {
						return nil, err
					}
				}

				// load URL spec first to remove URI key from the spec
				kindsFromURL, err = loader.LoadSpecs(ctx, loader.LoadOptions{
					RawSpec: specFromURL,
				})
				if err != nil {
					return nil, err
				}
				// remove URI key from the spec if any
				for i := range kindsFromURL.SupportBundlesV1Beta2 {
					kindsFromURL.SupportBundlesV1Beta2[i].Spec.Uri = ""
				}

			}
		}
	}

	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: rawSpecs,
	})
	if err != nil {
		return nil, err
	}
	if kindsFromURL != nil {
		kinds.Add(kindsFromURL)
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

func downloadFromHttpURL(ctx context.Context, url string, headers map[string]string) (string, error) {
	hs := []string{}
	for k, v := range headers {
		hs = append(hs, fmt.Sprintf("%s: %s", k, v))
	}

	klog.V(1).Infof("Downloading troubleshoot specs: url=%s, headers=[%v]", url, strings.Join(hs, ", "))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		// exit code: should this be catch all or spec issues...?
		return "", types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// exit code: should this be catch all or spec issues...?
		return "", types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, err)
	}
	defer resp.Body.Close()

	klog.V(1).Infof("Response status: %s", resp.Status)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
	}
	return string(body), nil
}

// LoadFromCluster loads troubleshoot specs from the cluster based on the provided labels.
// By default this will be troubleshoot.io/kind=support-bundle and troubleshoot.sh/kind=support-bundle
// labels. We search for secrets and configmaps with the label selector and extract the raw data.
// We then load the specs from the raw data. If the user does not have sufficient permissions
// to list & read secrets and configmaps from all namespaces, we will fallback to trying each
// namespace individually, and eventually default to the configured kubeconfig namespace.
func LoadFromCluster(ctx context.Context, client kubernetes.Interface, selectors []string, ns string) (*loader.TroubleshootKinds, error) {
	klog.V(1).Infof("Load troubleshoot specs from the cluster using selectors: %v", selectors)

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

// LoadAdditionalSpecFromURIs loads additional specs from the URIs provided in the kinds.
// This function will modify kinds in place.
func LoadAdditionalSpecFromURIs(ctx context.Context, kinds *loader.TroubleshootKinds) {

	obj := reflect.ValueOf(*kinds)

	// iterate over all fields of the TroubleshootKinds
	// e.g. SupportBundlesV1Beta2, PreflightsV1Beta2, etc.
	for i := 0; i < obj.NumField(); i++ {
		field := obj.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}

		// look at each spec in the slice
		// e.g. each spec in []PreflightsV1Beta2
		for count := 0; count < field.Len(); count++ {
			currentSpec := field.Index(count)
			specName := currentSpec.Type().Name()

			// check if .Spec.Uri exists
			specField := currentSpec.FieldByName("Spec")
			if !specField.IsValid() {
				continue
			}
			uriField := specField.FieldByName("Uri")
			if uriField.Kind() != reflect.String {
				continue
			}

			// download spec from URI
			uri := uriField.String()
			if uri == "" {
				continue
			}
			rawSpec, err := downloadFromHttpURL(ctx, uri, nil)
			if err != nil {
				klog.Warningf("failed to download spec from URI %q: %v", uri, err)
				continue
			}

			// load spec from raw spec
			uriKinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{RawSpec: string(rawSpec)})
			if err != nil {
				klog.Warningf("failed to load spec from URI %q: %v", uri, err)
				continue
			}

			// replace original spec with the loaded spec from URI
			newSpec := getFirstSpecOf(uriKinds, specName)
			if !newSpec.IsValid() {
				klog.Warningf("failed to read spec of type %s in URI %s", specName, uri)
				continue
			}
			currentSpec.Set(newSpec)
		}
	}
}

// dynamically get spec from kinds of given name
// return first element of the spec slice
func getFirstSpecOf(kinds *loader.TroubleshootKinds, name string) reflect.Value {
	obj := reflect.ValueOf(*kinds)
	for i := 0; i < obj.NumField(); i++ {
		field := obj.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}
		if field.Len() > 0 {
			if field.Index(0).Type().Name() == name {
				// return first element
				return field.Index(0)
			}
		}
	}
	return reflect.Value{}
}

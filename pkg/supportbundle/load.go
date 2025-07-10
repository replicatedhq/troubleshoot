package supportbundle

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
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// GetSupportBundleFromURI downloads and parses a support bundle from a URI and returns a SupportBundle object
func GetSupportBundleFromURI(bundleURI string) (*troubleshootv1beta2.SupportBundle, error) {
	collectorContent, err := LoadSupportBundleSpec(bundleURI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load collector spec")
	}

	multidocs := strings.Split(string(collectorContent), "\n---\n")

	supportbundle, err := ParseSupportBundle([]byte(multidocs[0]), true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse collector")
	}

	return supportbundle, nil
}

// ParseSupportBundle parses a support bundle from a byte array into a SupportBundle object
func ParseSupportBundle(doc []byte, followURI bool) (*troubleshootv1beta2.SupportBundle, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode(doc, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse document")
	}

	// parse doc and detect if it's a SupportBundle type,
	// if it's a Collector type, convert it to a SupportBundle

	collector, ok := obj.(*troubleshootv1beta2.Collector)
	if ok {
		supportBundle := troubleshootv1beta2.SupportBundle{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "troubleshoot.sh/v1beta2",
				Kind:       "SupportBundle",
			},
			ObjectMeta: collector.ObjectMeta,
			Spec: troubleshootv1beta2.SupportBundleSpec{
				Collectors:      collector.Spec.Collectors,
				HostCollectors:  collector.Spec.HostCollectors,
				Analyzers:       []*troubleshootv1beta2.Analyze{},
				HostAnalyzers:   []*troubleshootv1beta2.HostAnalyze{},
				AfterCollection: collector.Spec.AfterCollection,
			},
		}

		return &supportBundle, nil
	}

	supportBundle, ok := obj.(*troubleshootv1beta2.SupportBundle)
	if ok {
		// check if there is a uri field and if so,
		// use the upstream spec, otherwise fall back to
		// what's defined in the current spec
		if supportBundle.Spec.Uri != "" && followURI {
			klog.V(1).Infof("using upstream reference: %+v\n", supportBundle.Spec.Uri)
			upstreamSupportBundleContent, err := LoadSupportBundleSpec(supportBundle.Spec.Uri)
			if err != nil {
				klog.Errorf("failed to load upstream supportbundle, falling back")
				return supportBundle, nil
			}

			multidocs := strings.Split(string(upstreamSupportBundleContent), "\n---\n")

			upstreamSupportBundle, err := ParseSupportBundle([]byte(multidocs[0]), false)
			if err != nil {
				klog.Errorf("failed to parse upstream supportbundle, falling back")
				return supportBundle, nil
			}
			return upstreamSupportBundle, nil
		}
		return supportBundle, nil
	}

	return nil, errors.New("spec was not parseable as a troubleshoot kind")
}

// ParseSupportBundle parses a support bundle from a byte array into a SupportBundle object
// We will deprecate this in favour of use loader.LoadSpecs once the new API is stable
func ParseSupportBundleFromDoc(doc []byte) (*troubleshootv1beta2.SupportBundle, error) {
	return ParseSupportBundle(doc, true)
}

// GetRedactorFromURI parses a redactor from a URI into a Redactor object
// We will deprecate this in favour of use loader.LoadSpecs once the new API is stable
func GetRedactorFromURI(redactorURI string) (*troubleshootv1beta2.Redactor, error) {
	redactorContent, err := LoadRedactorSpec(redactorURI)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load redactor spec %s", redactorURI)
	}

	redactor, ok, err := toRedactGVK([]byte(redactorContent))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse redactor from doc")
	}
	if !ok {
		return nil, fmt.Errorf("%s is not a troubleshootv1beta2 redactor type", redactorURI)
	}

	return redactor, nil
}

// GetRedactorsFromURIs parses redactors from a URIs Redactor objects
// We will deprecate this in favour of use loader.LoadSpecs once the new API is stable
func GetRedactorsFromURIs(redactorURIs []string) ([]*troubleshootv1beta2.Redact, error) {
	redactors := []*troubleshootv1beta2.Redact{}
	for _, redactor := range redactorURIs {
		redactorObj, err := GetRedactorFromURI(redactor)
		if err != nil {
			return nil, err
		}

		if redactorObj != nil {
			redactors = append(redactors, redactorObj.Spec.Redactors...)
		}
	}

	return redactors, nil
}

func LoadSupportBundleSpec(arg string) ([]byte, error) {
	if strings.HasPrefix(arg, "secret/") {
		// format secret/namespace-name/secret-name
		pathParts := strings.Split(arg, "/")
		if len(pathParts) != 3 {
			return nil, errors.Errorf("secret path %s must have 3 components", arg)
		}

		spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "support-bundle-spec")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get spec from secret")
		}

		return spec, nil
	}

	return loadSpec(arg)
}

func LoadRedactorSpec(arg string) ([]byte, error) {
	if strings.HasPrefix(arg, "configmap/") {
		// format configmap/namespace-name/configmap-name[/data-key]
		pathParts := strings.Split(arg, "/")
		if len(pathParts) > 4 {
			return nil, errors.Errorf("configmap path %s must have at most 4 components", arg)
		}
		if len(pathParts) < 3 {
			return nil, errors.Errorf("configmap path %s must have at least 3 components", arg)
		}

		dataKey := "redactor-spec"
		if len(pathParts) == 4 {
			dataKey = pathParts[3]
		}

		spec, err := specs.LoadFromConfigMap(pathParts[1], pathParts[2], dataKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get spec from configmap")
		}

		return spec, nil
	}

	spec, err := loadSpec(arg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load spec")
	}

	return spec, nil
}

func loadSpec(arg string) ([]byte, error) {
	var err error
	if _, err = os.Stat(arg); err == nil {
		b, err := os.ReadFile(arg)
		if err != nil {
			return nil, errors.Wrap(err, "read spec file")
		}

		return b, nil
	}

	u, err := url.Parse(arg)
	if err != nil {
		return nil, errors.Wrapf(err, "%s is not a valid URL (%s)", arg, err)
	}

	if u.Scheme == "oci" {
		content, err := oci.PullSupportBundleFromOCI(arg)
		if err != nil {
			if err == oci.ErrNoRelease {
				return nil, errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", arg)
			}

			return nil, errors.Wrap(err, "pull from oci")
		}

		return content, nil
	}

	if !util.IsURL(arg) {
		return nil, fmt.Errorf("%s is not a URL and was not found", arg)
	}

	spec, err := loadSpecFromURL(arg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get spec from URL")
	}
	return spec, nil
}

func loadSpecFromURL(arg string) ([]byte, error) {
	for {
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return nil, errors.Wrap(err, "make request")
		}
		req.Header.Set("User-Agent", "Replicated_Troubleshoot/v1beta1")
		req.Header.Set("Bundle-Upload-Host", fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host))
		httpClient := httputil.GetHttpClient()
		resp, err := httpClient.Do(req)
		if err != nil {
			if shouldRetryRequest(err) {
				continue
			}
			return nil, errors.Wrap(err, "execute request")
		}

		// handle non 2xx http statuses
		// redirects appear to already be handled by the go http client
		// TODO: handle potential for redirect loops breaking this?
		if resp.StatusCode != 200 {
			return nil, errors.New("request returned non 200 response")
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "read responce body")
		}

		return body, nil
	}
}

// ParseRedactorsFromDocs parses a slice of YAML docs and returns a slice of Redactors
// We will deprecate this in favour of use loader.LoadSpecs once the new API is stable
func ParseRedactorsFromDocs(docs []string) ([]*troubleshootv1beta2.Redact, error) {
	var redactors []*troubleshootv1beta2.Redact

	for i, doc := range docs {
		multidocRedactors, ok, err := toRedactGVK([]byte(doc))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse redactor from doc %d", i)
		}
		if !ok {
			continue
		}

		redactors = append(redactors, multidocRedactors.Spec.Redactors...)
	}

	return redactors, nil
}

func toRedactGVK(doc []byte) (*troubleshootv1beta2.Redactor, bool, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(doc), nil, nil)
	if err != nil {
		return nil, false, err
	}

	multidocRedactors, ok := obj.(*troubleshootv1beta2.Redactor)
	return multidocRedactors, ok, nil
}

package loader

import (
	"context"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootv1beta3 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta3"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

var decoder runtime.Decoder

func init() {
	// Allow serializing Secrets and ConfigMaps
	_ = v1.AddToScheme(scheme.Scheme)
	decoder = scheme.Codecs.UniversalDeserializer()
}

type parsedDoc struct {
	Kind       string            `json:"kind" yaml:"kind"`
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Data       map[string]any    `json:"data" yaml:"data"`
	StringData map[string]string `json:"stringData" yaml:"stringData"`
}

type LoadOptions struct {
	RawSpecs []string
	RawSpec  string

	// If true, the loader will return an error if any of the specs are not valid
	// else the invalid specs will be ignored
	Strict bool

	// Client is the kubernetes client used for resolving v1beta3 StringOrValueFrom fields
	// If not provided, v1beta3 specs with secretKeyRef will fail to load
	Client kubernetes.Interface

	// Namespace is the default namespace for resolving v1beta3 secret references
	// Defaults to "default" if not provided
	Namespace string
}

// TODO: Additional requirements needed in this package
// * Downloading specs from remote locations e.g oci, s3, http etc
// 	 * Remote connection error handing
//   * Support various auth methods
//   * Retry logic and how to handle timeouts
// * Support for loading specs from paths e.g directory, file, stdin, tarballs, zips etc
// * Support for loading specs from a kubernetes cluster - concrete use case of remote location

// LoadSpecs takes sources to load specs from and returns a TroubleshootKinds object
// that contains all the parsed troubleshoot specs.
//
// The fetched specs should be yaml documents. The documents can be multidoc yamls
// separated by "---" which get split and parsed one at a time. All troubleshoot
// specs are extracted from the documents and returned in a TroubleshootKinds object.
//
// If Secrets or ConfigMaps are found, they are parsed and the support bundle, redactor
// or preflight spec extracted from them. All other yaml documents will be ignored.
//
// If the `Strict` flag is set to true, this function will return an error if any of
// the documents are not valid, else the invalid documents will be ignored.
func LoadSpecs(ctx context.Context, opt LoadOptions) (*TroubleshootKinds, error) {
	opt.RawSpecs = append(opt.RawSpecs, opt.RawSpec)

	// Default namespace to "default" if not provided
	namespace := opt.Namespace
	if namespace == "" {
		namespace = "default"
	}

	l := specLoader{
		strict:    opt.Strict,
		client:    opt.Client,
		namespace: namespace,
		ctx:       ctx,
	}

	return l.loadFromStrings(opt.RawSpecs...)
}

type TroubleshootKinds struct {
	AnalyzersV1Beta2        []troubleshootv1beta2.Analyzer
	CollectorsV1Beta2       []troubleshootv1beta2.Collector
	HostCollectorsV1Beta2   []troubleshootv1beta2.HostCollector
	HostPreflightsV1Beta2   []troubleshootv1beta2.HostPreflight
	PreflightsV1Beta2       []troubleshootv1beta2.Preflight
	RedactorsV1Beta2        []troubleshootv1beta2.Redactor
	RemoteCollectorsV1Beta2 []troubleshootv1beta2.RemoteCollector
	SupportBundlesV1Beta2   []troubleshootv1beta2.SupportBundle
}

func (kinds *TroubleshootKinds) IsEmpty() bool {
	return kinds.Len() == 0
}

func (kinds *TroubleshootKinds) Len() int {
	return len(kinds.AnalyzersV1Beta2) +
		len(kinds.CollectorsV1Beta2) +
		len(kinds.HostCollectorsV1Beta2) +
		len(kinds.HostPreflightsV1Beta2) +
		len(kinds.PreflightsV1Beta2) +
		len(kinds.RedactorsV1Beta2) +
		len(kinds.RemoteCollectorsV1Beta2) +
		len(kinds.SupportBundlesV1Beta2)
}

func (kinds *TroubleshootKinds) Add(other *TroubleshootKinds) {
	kinds.AnalyzersV1Beta2 = append(kinds.AnalyzersV1Beta2, other.AnalyzersV1Beta2...)
	kinds.CollectorsV1Beta2 = append(kinds.CollectorsV1Beta2, other.CollectorsV1Beta2...)
	kinds.HostCollectorsV1Beta2 = append(kinds.HostCollectorsV1Beta2, other.HostCollectorsV1Beta2...)
	kinds.HostPreflightsV1Beta2 = append(kinds.HostPreflightsV1Beta2, other.HostPreflightsV1Beta2...)
	kinds.PreflightsV1Beta2 = append(kinds.PreflightsV1Beta2, other.PreflightsV1Beta2...)
	kinds.RedactorsV1Beta2 = append(kinds.RedactorsV1Beta2, other.RedactorsV1Beta2...)
	kinds.RemoteCollectorsV1Beta2 = append(kinds.RemoteCollectorsV1Beta2, other.RemoteCollectorsV1Beta2...)
	kinds.SupportBundlesV1Beta2 = append(kinds.SupportBundlesV1Beta2, other.SupportBundlesV1Beta2...)
}

// ToYaml returns a yaml document/multi-doc of all the parsed specs
// This function utilises reflection to iterate over all the fields
// of the TroubleshootKinds object then marshals them to yaml.
func (kinds *TroubleshootKinds) ToYaml() (string, error) {
	rawList := []string{}
	obj := reflect.ValueOf(*kinds)

	for i := 0; i < obj.NumField(); i++ {
		field := obj.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}

		// skip empty slices to avoid empty yaml documents
		for count := 0; count < field.Len(); count++ {
			val := field.Index(count)
			yamlOut, err := yaml.Marshal(val.Interface())
			if err != nil {
				return "", err
			}
			rawList = append(rawList, string(yamlOut))
		}
	}

	return strings.Join(rawList, "---\n"), nil
}

func NewTroubleshootKinds() *TroubleshootKinds {
	return &TroubleshootKinds{}
}

type specLoader struct {
	strict    bool
	client    kubernetes.Interface
	namespace string
	ctx       context.Context
}

// loadFromStrings accepts a list of strings (exploded) which should be yaml documents
func (l *specLoader) loadFromStrings(rawSpecs ...string) (*TroubleshootKinds, error) {
	splitdocs := []string{}
	multiRawDocs := []string{}

	// 1. First split multidoc yaml documents.
	for _, rawSpec := range rawSpecs {
		multiRawDocs = append(multiRawDocs, util.SplitYAML(rawSpec)...)
	}

	// 2. Go through each document to see if it is a configmap, secret or troubleshoot kind
	// For secrets and configmaps, extract support bundle, redactor or preflight specs
	// For troubleshoot kinds, pass them through
	for _, rawDoc := range multiRawDocs {
		if rawDoc == "" {
			continue
		}

		var parsed parsedDoc

		err := yaml.Unmarshal([]byte(rawDoc), &parsed)
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrapf(err, "failed to parse yaml: '%s'", string(rawDoc)))
		}

		if isConfigMap(parsed) || isSecret(parsed) {
			// Extract specs from configmap or secret
			obj, _, err := decoder.Decode([]byte(rawDoc), nil, nil)
			if err != nil {
				if !l.strict {
					continue
				}
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
					errors.Wrapf(err, "failed to decode raw spec: '%s'", string(rawDoc)),
				)
			}

			// 3. Extract the raw troubleshoot specs
			switch v := obj.(type) {
			case *v1.ConfigMap:
				specs, err := l.getSpecFromConfigMap(v)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}
				splitdocs = append(splitdocs, specs...)
			case *v1.Secret:
				specs, err := l.getSpecFromSecret(v)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}
				splitdocs = append(splitdocs, specs...)
			default:
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("%T type is not a Secret or ConfigMap", v))
			}
		} else if parsed.APIVersion == constants.Troubleshootv1beta3Kind || parsed.APIVersion == constants.Troubleshootv1beta2Kind || parsed.APIVersion == constants.Troubleshootv1beta1Kind {
			// If it's not a configmap or secret, just append it to the splitdocs
			splitdocs = append(splitdocs, rawDoc)
		} else {
			klog.V(2).Infof("Skip loading %q kind", parsed.Kind)
		}
	}

	// 4. Then load the specs into the kinds struct
	return l.loadFromSplitDocs(splitdocs)
}

func (l *specLoader) loadFromSplitDocs(splitdocs []string) (*TroubleshootKinds, error) {
	kinds := NewTroubleshootKinds()

	for _, doc := range splitdocs {
		// Check if this is a v1beta3 spec
		var parsed parsedDoc
		if err := yaml.Unmarshal([]byte(doc), &parsed); err == nil && parsed.APIVersion == constants.Troubleshootv1beta3Kind {
			// Only handle v1beta3 SupportBundle specially (to resolve valueFrom and convert)
			if strings.EqualFold(parsed.Kind, "SupportBundle") {
				if err := l.loadV1Beta3Spec(doc, kinds); err != nil {
					if !l.strict {
						klog.Warningf("failed to load v1beta3 support bundle spec: %v", err)
						continue
					}
					return nil, err
				}
				// handled as support bundle; move to next doc
				continue
			}
			// For other v1beta3 kinds (e.g., Preflight), fall through to the generic
			// v1beta3->v1beta2 conversion path below to preserve prior behavior.
		}

		// Handle v1beta2 and v1beta1 specs
		converted, err := docrewrite.ConvertToV1Beta2([]byte(doc))
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.Wrapf(err, "failed to convert doc to troubleshoot.sh/v1beta2 kind: '\n%s'", doc),
			)
		}

		obj, _, err := decoder.Decode([]byte(converted), nil, nil)
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.Wrapf(err, "failed to decode '%s'", converted),
			)
		}

		switch spec := obj.(type) {
		case *troubleshootv1beta2.Analyzer:
			kinds.AnalyzersV1Beta2 = append(kinds.AnalyzersV1Beta2, *spec)
		case *troubleshootv1beta2.Collector:
			kinds.CollectorsV1Beta2 = append(kinds.CollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.HostCollector:
			kinds.HostCollectorsV1Beta2 = append(kinds.HostCollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.HostPreflight:
			kinds.HostPreflightsV1Beta2 = append(kinds.HostPreflightsV1Beta2, *spec)
		case *troubleshootv1beta2.Preflight:
			kinds.PreflightsV1Beta2 = append(kinds.PreflightsV1Beta2, *spec)
		case *troubleshootv1beta2.Redactor:
			kinds.RedactorsV1Beta2 = append(kinds.RedactorsV1Beta2, *spec)
		case *troubleshootv1beta2.RemoteCollector:
			kinds.RemoteCollectorsV1Beta2 = append(kinds.RemoteCollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.SupportBundle:
			kinds.SupportBundlesV1Beta2 = append(kinds.SupportBundlesV1Beta2, *spec)
		default:
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("unknown troubleshoot kind %T", obj))
		}
	}

	klog.V(2).Infof("Loaded %d troubleshoot specs successfully", kinds.Len())

	return kinds, nil
}

// loadV1Beta3Spec handles loading and resolving v1beta3 specs
func (l *specLoader) loadV1Beta3Spec(doc string, kinds *TroubleshootKinds) error {
	// Unmarshal to v1beta3 types
	obj, _, err := decoder.Decode([]byte(doc), nil, nil)
	if err != nil {
		return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
			errors.Wrapf(err, "failed to decode v1beta3 spec: '%s'", doc),
		)
	}

	switch v3spec := obj.(type) {
	case *troubleshootv1beta3.SupportBundle:
		// Resolve secrets and convert to v1beta2
		requiresClient := v1beta3SpecRequiresClient(&v3spec.Spec)
		if requiresClient && l.client == nil {
			return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.New("kubernetes client required to resolve v1beta3 specs with secret/configmap references"),
			)
		}

		v2spec, err := troubleshootv1beta3.ConvertToV1Beta2WithResolution(l.ctx, &v3spec.Spec, l.client, l.namespace)
		if err != nil {
			return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.Wrap(err, "failed to resolve and convert v1beta3 support bundle spec"),
			)
		}

		// Create v1beta2 support bundle
		v2bundle := troubleshootv1beta2.SupportBundle{
			TypeMeta:   v3spec.TypeMeta,
			ObjectMeta: v3spec.ObjectMeta,
			Spec:       *v2spec,
		}
		// Update apiVersion to v1beta2
		v2bundle.APIVersion = constants.Troubleshootv1beta2Kind
		kinds.SupportBundlesV1Beta2 = append(kinds.SupportBundlesV1Beta2, v2bundle)

	// TODO: Add other v1beta3 types as they are implemented
	default:
		return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
			errors.Errorf("unsupported v1beta3 kind: %T", v3spec),
		)
	}

	return nil
}

// v1beta3SpecRequiresClient returns true if the v1beta3 spec contains any
// StringOrValueFrom references that require fetching from the cluster.
func v1beta3SpecRequiresClient(spec *troubleshootv1beta3.SupportBundleSpec) bool {
	if spec == nil || spec.Collectors == nil {
		return false
	}

	for _, c := range spec.Collectors {
		if c == nil {
			continue
		}

		// Database collectors
		if c.Postgres != nil && databaseRequiresClient(c.Postgres) {
			return true
		}
		if c.Mysql != nil && databaseRequiresClient(c.Mysql) {
			return true
		}
		if c.Mssql != nil && databaseRequiresClient(c.Mssql) {
			return true
		}
		if c.Redis != nil && databaseRequiresClient(c.Redis) {
			return true
		}
	}

	return false
}

func databaseRequiresClient(db *troubleshootv1beta3.Database) bool {
	if db == nil {
		return false
	}

	if stringOrValueFromHasRef(db.URI) {
		return true
	}

	if db.TLS != nil {
		if stringOrValueFromHasRef(db.TLS.CACert) ||
			stringOrValueFromHasRef(db.TLS.ClientCert) ||
			stringOrValueFromHasRef(db.TLS.ClientKey) {
			return true
		}
	}

	return false
}

func stringOrValueFromHasRef(s troubleshootv1beta3.StringOrValueFrom) bool {
	if s.ValueFrom == nil {
		return false
	}
	return s.ValueFrom.SecretKeyRef != nil || s.ValueFrom.ConfigMapKeyRef != nil
}

func isSecret(parsedDocHead parsedDoc) bool {
	if parsedDocHead.Kind == "Secret" && parsedDocHead.APIVersion == "v1" {
		return true
	}

	return false
}

func isConfigMap(parsedDocHead parsedDoc) bool {
	if parsedDocHead.Kind == "ConfigMap" && parsedDocHead.APIVersion == "v1" {
		return true
	}

	return false
}

// getSpecFromConfigMap extracts multiple troubleshoot specs from a secret
func (l *specLoader) getSpecFromConfigMap(cm *v1.ConfigMap) ([]string, error) {
	// TODO: Consider not checking for the existence of the key and just trying to decode
	specs := []string{}

	str, ok := cm.Data[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = cm.Data[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = cm.Data[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = cm.Data[constants.PreflightKey2]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}

	return specs, nil
}

// getSpecFromSecret extracts multiple troubleshoot specs from a secret
func (l *specLoader) getSpecFromSecret(secret *v1.Secret) ([]string, error) {
	// TODO: Consider not checking for the existence of the key and just trying to decode
	specs := []string{}

	specBytes, ok := secret.Data[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	specBytes, ok = secret.Data[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	specBytes, ok = secret.Data[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	specBytes, ok = secret.Data[constants.PreflightKey2]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	str, ok := secret.StringData[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = secret.StringData[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = secret.StringData[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = secret.StringData[constants.PreflightKey2]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	return specs, nil
}

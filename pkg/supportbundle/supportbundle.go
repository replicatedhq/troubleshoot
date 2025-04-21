package supportbundle

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/internal/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"go.opentelemetry.io/otel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type SupportBundleCreateOpts struct {
	CollectorProgressCallback func(chan interface{}, string)
	CollectWithoutPermissions bool
	HttpClient                *http.Client
	KubernetesRestConfig      *rest.Config
	Namespace                 string
	ProgressChan              chan interface{}
	SinceTime                 *time.Time
	OutputPath                string
	Redact                    bool
	FromCLI                   bool
	RunHostCollectorsInPod    bool
}

type SupportBundleResponse struct {
	AnalyzerResults []*analyzer.AnalyzeResult
	ArchivePath     string
	FileUploaded    bool
}

// NodeList is a list of remote nodes to collect data from in a support bundle
type NodeList struct {
	Nodes []string `json:"nodes"`
}

// CollectSupportBundleFromSpec collects support bundle from start to finish, including running
// collectors, analyzers and after collection steps. Input arguments are specifications.
// if FromCLI option is set to true, the output is the name of the archive on disk in the cwd.
// if FromCLI option is set to false, the support bundle is archived in the OS temp folder (os.TempDir()).
func CollectSupportBundleFromSpec(
	spec *troubleshootv1beta2.SupportBundleSpec, additionalRedactors *troubleshootv1beta2.Redactor, opts SupportBundleCreateOpts,
) (*SupportBundleResponse, error) {

	resultsResponse := SupportBundleResponse{}

	if opts.KubernetesRestConfig == nil {
		return nil, errors.New("did not receive kube rest config")
	}

	if opts.ProgressChan == nil {
		return nil, errors.New("did not receive collector progress chan")
	}

	tmpDir, err := os.MkdirTemp("", "supportbundle")
	if err != nil {
		return nil, errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(tmpDir)
	klog.V(2).Infof("Support bundle created in temporary directory: %s", tmpDir)

	basename := ""
	if opts.OutputPath != "" {
		// use override output path
		overridePath, err := convert.ValidateOutputPath(opts.OutputPath)
		if err != nil {
			return nil, errors.Wrap(err, "override output file path")
		}
		basename = strings.TrimSuffix(overridePath, ".tar.gz")
	} else {
		// use default output path
		basename = fmt.Sprintf("support-bundle-%s", time.Now().Format("2006-01-02T15_04_05"))
		if !opts.FromCLI {
			basename = filepath.Join(os.TempDir(), basename)
		}
	}

	filename, err := findFileName(basename, "tar.gz")
	if err != nil {
		return nil, errors.Wrap(err, "find file name")
	}
	resultsResponse.ArchivePath = filename

	bundlePath := filepath.Join(tmpDir, strings.TrimSuffix(filename, ".tar.gz"))
	if err := os.MkdirAll(bundlePath, 0777); err != nil {
		return nil, errors.Wrap(err, "create bundle dir")
	}

	result := make(collect.CollectorResult)

	ctx, root := otel.Tracer(constants.LIB_TRACER_NAME).Start(
		context.Background(), constants.TROUBLESHOOT_ROOT_SPAN_NAME,
	)
	defer func() {
		// If this function returns an error, root.End() may not be called.
		// We want to ensure this happens, so we defer it. It is safe to call
		// root.End() multiple times.
		root.End()
	}()

	// Cache error returned by collectors and return it at the end of the function
	// so as to have a chance to run analyzers and archive the support bundle after.
	// If both host and in cluster collectors fail, the errors will be wrapped
	collectorsErrs := []string{}
	var files, hostFiles collect.CollectorResult

	if spec.HostCollectors != nil {
		// Run host collectors
		hostFiles, err = runHostCollectors(ctx, spec.HostCollectors, additionalRedactors, bundlePath, opts)
		if err != nil {
			collectorsErrs = append(collectorsErrs, fmt.Sprintf("failed to run host collectors: %s", err))
		}
	}

	if spec.Collectors != nil {
		// Run collectors
		files, err = runCollectors(ctx, spec.Collectors, additionalRedactors, bundlePath, opts)
		if err != nil {
			collectorsErrs = append(collectorsErrs, fmt.Sprintf("failed to run collectors: %s", err))
		}
	}

	// merge in-cluster and host collectors results
	for k, v := range files {
		result[k] = v
	}

	for k, v := range hostFiles {
		result[k] = v
	}

	if len(result) == 0 {
		if len(collectorsErrs) > 0 {
			return nil, fmt.Errorf("failed to generate support bundle: %s", strings.Join(collectorsErrs, "\n"))
		}
		return nil, fmt.Errorf("failed to generate support bundle")
	}

	version, err := version.GetVersionFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get version file")
	}

	err = result.SaveResult(bundlePath, constants.VERSION_FILENAME, bytes.NewBuffer([]byte(version)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write version")
	}

	// Run Analyzers
	analyzeResults, err := AnalyzeSupportBundle(ctx, spec, bundlePath)
	if err != nil {
		if opts.FromCLI {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * %v\n", cursor.ClearEntireLine(), err)
			// don't die
		} else {
			return nil, errors.Wrap(err, "failed to run analysis")
		}
	}
	resultsResponse.AnalyzerResults = analyzeResults

	analysis, err := getAnalysisFile(analyzeResults)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get analysis file")
	}

	err = result.SaveResult(bundlePath, constants.ANALYSIS_FILENAME, analysis)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write analysis")
	}

	// Complete tracing by ending the root span and collecting
	// the summary of the traces. Store them in the support bundle.
	root.End()
	summary := traces.GetExporterInstance().GetSummary()
	err = result.SaveResult(bundlePath, "execution-data/summary.txt", bytes.NewReader([]byte(summary)))
	if err != nil {
		// Don't fail the support bundle if we can't save the execution summary
		klog.Errorf("failed to save execution summary file in the support bundle: %v", err)
	}

	// Archive Support Bundle
	if err := result.ArchiveBundle(bundlePath, filename); err != nil {
		return nil, errors.Wrap(err, "create bundle file")
	}

	fileUploaded, err := ProcessSupportBundleAfterCollection(spec, filename)
	if err != nil {
		if opts.FromCLI {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * %v\n", cursor.ClearEntireLine(), err)
			// don't die
		} else {
			return nil, errors.Wrap(err, "failed to process bundle after collection")
		}
	}
	resultsResponse.FileUploaded = fileUploaded

	if len(collectorsErrs) > 0 {
		// TODO: Consider a collectors error type
		// TODO: use errors.Join in go 1.20 (https://pkg.go.dev/errors#Join)
		return &resultsResponse, fmt.Errorf("%s", strings.Join(collectorsErrs, "\n"))
	}

	return &resultsResponse, nil
}

// CollectSupportBundleFromURI collects support bundle from start to finish, including running
// collectors, analyzers and after collection steps. Input arguments are the URIs of the support bundle and redactor specs.
// The support bundle is archived in the OS temp folder (os.TempDir()).
func CollectSupportBundleFromURI(specURI string, redactorURIs []string, opts SupportBundleCreateOpts) (*SupportBundleResponse, error) {
	supportBundle, err := GetSupportBundleFromURI(specURI)
	if err != nil {
		return nil, errors.Wrap(err, "could not bundle from URI")
	}

	redactors, err := GetRedactorsFromURIs(redactorURIs)
	if err != nil {
		return nil, err
	}
	additionalRedactors := &troubleshootv1beta2.Redactor{}
	additionalRedactors.Spec.Redactors = redactors

	return CollectSupportBundleFromSpec(&supportBundle.Spec, additionalRedactors, opts)
}

// ProcessSupportBundleAfterCollection performs the after collection actions, like Callbacks and sending the archive to a remote server.
func ProcessSupportBundleAfterCollection(spec *troubleshootv1beta2.SupportBundleSpec, archivePath string) (bool, error) {
	fileUploaded := false
	if len(spec.AfterCollection) > 0 {
		for _, ac := range spec.AfterCollection {
			if ac.UploadResultsTo != nil {
				if err := uploadSupportBundle(ac.UploadResultsTo, archivePath); err != nil {
					return false, errors.Wrap(err, "failed to upload support bundle")
				} else {
					fileUploaded = true
				}
			} else if ac.Callback != nil {
				if err := callbackSupportBundleAPI(ac.Callback, archivePath); err != nil {
					return false, errors.Wrap(err, "failed to notify API that support bundle has been uploaded")
				}
			}
		}
	}
	return fileUploaded, nil
}

// AnalyzeSupportBundle performs analysis on a support bundle using the support bundle spec and an already unpacked support
// bundle on disk
func AnalyzeSupportBundle(ctx context.Context, spec *troubleshootv1beta2.SupportBundleSpec, tmpDir string) ([]*analyzer.AnalyzeResult, error) {
	if len(spec.Analyzers) == 0 && len(spec.HostAnalyzers) == 0 {
		return nil, nil
	}
	spec.Analyzers = analyzer.DedupAnalyzers(spec.Analyzers)
	analyzeResults, err := analyzer.AnalyzeLocal(ctx, tmpDir, spec.Analyzers, spec.HostAnalyzers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze support bundle")
	}
	return analyzeResults, nil
}

// ConcatSpec the intention with these appends is to swap them out at a later date with more specific handlers for merging the spec fields
func ConcatSpec(target *troubleshootv1beta2.SupportBundle, source *troubleshootv1beta2.SupportBundle) *troubleshootv1beta2.SupportBundle {
	if source == nil {
		return target
	}
	var newBundle *troubleshootv1beta2.SupportBundle
	if target == nil {
		newBundle = source
	} else {
		newBundle = target.DeepCopy()
		newBundle.Spec.Collectors = util.Append(target.Spec.Collectors, source.Spec.Collectors)
		newBundle.Spec.AfterCollection = util.Append(target.Spec.AfterCollection, source.Spec.AfterCollection)
		newBundle.Spec.HostCollectors = util.Append(target.Spec.HostCollectors, source.Spec.HostCollectors)
		newBundle.Spec.HostAnalyzers = util.Append(target.Spec.HostAnalyzers, source.Spec.HostAnalyzers)
		newBundle.Spec.Analyzers = util.Append(target.Spec.Analyzers, source.Spec.Analyzers)
		// TODO: What to do with the Uri field?
	}
	return newBundle
}

func getNodeList(clientset kubernetes.Interface, opts SupportBundleCreateOpts) (*NodeList, error) {
	// todo: any node filtering on opts?
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list nodes")
	}

	nodeList := NodeList{}
	for _, node := range nodes.Items {
		nodeList.Nodes = append(nodeList.Nodes, node.Name)
	}

	return &nodeList, nil
}

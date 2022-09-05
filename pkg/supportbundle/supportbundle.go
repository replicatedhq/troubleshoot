package supportbundle

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"k8s.io/client-go/rest"
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
}

type SupportBundleResponse struct {
	AnalyzerResults []*analyzer.AnalyzeResult
	ArchivePath     string
	FileUploaded    bool
}

// CollectSupportBundleFromSpec collects support bundle from start to finish, including running
// collectors, analyzers and after collection steps. Input arguments are specifications.
// if FromCLI option is set to true, the output is the name of the archive on disk in the cwd.
// if FromCLI option is set to false, the support bundle is archived in the OS temp folder (os.TempDir()).
func CollectSupportBundleFromSpec(spec *troubleshootv1beta2.SupportBundleSpec, additionalRedactors *troubleshootv1beta2.Redactor, opts SupportBundleCreateOpts) (*SupportBundleResponse, error) {
	resultsResponse := SupportBundleResponse{}

	if opts.KubernetesRestConfig == nil {
		return nil, errors.New("did not receive kube rest config")
	}

	if opts.ProgressChan == nil {
		return nil, errors.New("did not receive collector progress chan")
	}

	tmpDir, err := ioutil.TempDir("", "supportbundle")
	if err != nil {
		return nil, errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(tmpDir)

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

	var result, files, hostFiles collect.CollectorResult

	if spec.HostCollectors != nil {
		// Run host collectors
		hostFiles, err = runHostCollectors(spec.HostCollectors, additionalRedactors, bundlePath, opts)
		if err != nil {
			fmt.Println(errors.Wrap(err, "failed to run host collectors"))
		}
	}

	if spec.Collectors != nil {
		// Run collectors
		files, err = runCollectors(spec.Collectors, additionalRedactors, bundlePath, opts)
		if err != nil {
			fmt.Println(errors.Wrap(err, "failed to run collectors"))
		}
	}

	if files != nil && hostFiles != nil {
		result = files
		for k, v := range hostFiles {
			result[k] = v
		}
	} else if files != nil {
		result = files
	} else if hostFiles != nil {
		result = hostFiles
	} else {
		return nil, errors.Wrap(err, "failed to generate support bundle")
	}

	version, err := getVersionFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get version file")
	}

	err = result.SaveResult(bundlePath, VersionFilename, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write version")
	}

	// Run Analyzers
	analyzeResults, err := AnalyzeSupportBundle(spec, bundlePath)
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

	err = result.SaveResult(bundlePath, AnalysisFilename, analysis)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write analysis")
	}

	if err := collect.TarSupportBundleDir(bundlePath, result, filename); err != nil {
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

	return &resultsResponse, nil
}

// CollectSupportBundleFromURI collects support bundle from start to finish, including running
// collectors, analyzers and after collection steps. Input arguments are the URIs of the support bundle and redactor specs.
// The support bundle is archived in the OS temp folder (os.TempDir()).
func CollectSupportBundleFromURI(specURI string, redactorURIs []string, opts SupportBundleCreateOpts) (*SupportBundleResponse, error) {
	supportbundle, err := GetSupportBundleFromURI(specURI)
	if err != nil {
		return nil, errors.Wrap(err, "could not bundle from URI")
	}

	additionalRedactors := &troubleshootv1beta2.Redactor{}
	for _, redactor := range redactorURIs {
		redactorObj, err := GetRedactorFromURI(redactor)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get redactor spec %s", redactor)
		}

		if redactorObj != nil {
			additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, redactorObj.Spec.Redactors...)
		}
	}

	return CollectSupportBundleFromSpec(&supportbundle.Spec, additionalRedactors, opts)
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
func AnalyzeSupportBundle(spec *troubleshootv1beta2.SupportBundleSpec, tmpDir string) ([]*analyzer.AnalyzeResult, error) {
	if len(spec.Analyzers) == 0 && len(spec.HostAnalyzers) == 0 {
		return nil, nil
	}
	analyzeResults, err := analyzer.AnalyzeLocal(tmpDir, spec.Analyzers, spec.HostAnalyzers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze support bundle")
	}
	return analyzeResults, nil
}


// the intention with these appends is to swap them out at a later date with more specific handlers for merging the spec fields
func ConcatSpec(target *troubleshootv1beta2.SupportBundle, source *troubleshootv1beta2.SupportBundle) *troubleshootv1beta2.SupportBundle{

	newBundle := target.DeepCopy()

	for _, v := range source.Spec.Collectors {
		newBundle.Spec.Collectors = append(target.Spec.Collectors,v)
	}

	for _, v := range source.Spec.AfterCollection {
		newBundle.Spec.AfterCollection = append(target.Spec.AfterCollection, v)
	}
	for _, v := range source.Spec.HostCollectors {
		newBundle.Spec.HostCollectors = append(target.Spec.HostCollectors, v)
	}
	for _, v := range source.Spec.HostAnalyzers {
		newBundle.Spec.HostAnalyzers = append(target.Spec.HostAnalyzers, v)
	}
	for _, v := range source.Spec.Analyzers {
		newBundle.Spec.Analyzers = append(target.Spec.Analyzers, v)
	}
	return newBundle
}

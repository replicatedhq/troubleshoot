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
	FromCLI                   bool
}

type SupportBundleResponse struct {
	AnalyzerResults []*analyzer.AnalyzeResult
	ArchivePath     string
	FileUploaded    bool
}

// CollectSupportBundleFromSpec collects support bundle from start to finish, including running
// collectors, analyzers and after collection steps. Input arguments are specifications.
// if FromCLI option is set to true, the output is the name of the archive on disk in the pwd.
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

	basename := filepath.Join(os.TempDir(), fmt.Sprintf("support-bundle-%s", time.Now().Format("2006-01-02T15_04_05")))
	if opts.FromCLI {
		basename = fmt.Sprintf("support-bundle-%s", time.Now().Format("2006-01-02T15_04_05"))
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

	if err = writeVersionFile(bundlePath); err != nil {
		return nil, errors.Wrap(err, "write version file")
	}

	// Run collectors
	err = runCollectors(spec.Collectors, additionalRedactors, filename, bundlePath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run collectors")
	}

	// Run Analyzers
	analyzeResults, err := AnalyzeSupportBundle(spec, tmpDir)
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

	// Add the analysis to the support bundle
	if err = writeAnalysisFile(bundlePath, analyzeResults); err != nil {
		return nil, errors.Wrap(err, "write version file")
	}

	if err := tarSupportBundleDir(bundlePath, filename); err != nil {
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
	if len(spec.Analyzers) == 0 {
		return nil, nil
	}
	analyzeResults, err := analyzer.AnalyzeLocal(tmpDir, spec.Analyzers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze support bundle")
	}
	return analyzeResults, nil
}

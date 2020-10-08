package analyzer

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	getter "github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"k8s.io/client-go/kubernetes/scheme"
)

type fileContentProvider struct {
	rootDir       string
	collectedData map[string][]byte
}

// Analyze local will analyze a locally available (already downloaded) bundle
func AnalyzeLocal(localBundlePath string, analyzers []*troubleshootv1beta2.Analyze, notRedactedData map[string][]byte) ([]*AnalyzeResult, error) {
	fcp := fileContentProvider{rootDir: localBundlePath, collectedData: notRedactedData}
	analyzeResults := []*AnalyzeResult{}
	for _, analyzer := range analyzers {
		analyzeResult, err := Analyze(analyzer, fcp.getFileContents, fcp.getChildFileContents)
		if err != nil {
			logger.Printf("an analyzer failed to run: %v\n", err)
			continue
		}

		if analyzeResult != nil {
			analyzeResults = append(analyzeResults, analyzeResult...)
		}
	}

	return analyzeResults, nil
}

func DownloadAndAnalyze(bundleURL string, analyzersSpec string) ([]*AnalyzeResult, error) {
	tmpDir, err := ioutil.TempDir("", "troubleshoot-k8s")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tmpDir)

	if err := downloadTroubleshootBundle(bundleURL, tmpDir); err != nil {
		return nil, errors.Wrap(err, "failed to download bundle")
	}

	_, err = os.Stat(filepath.Join(tmpDir, "version.yaml"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read version.yaml")
	}

	analyzers := []*troubleshootv1beta2.Analyze{}

	if analyzersSpec == "" {
		defaultAnalyzers, err := getDefaultAnalyzers()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get default analyzers")
		}
		analyzers = defaultAnalyzers
	} else {
		parsedAnalyzers, err := parseAnalyzers(analyzersSpec)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse analyzers")
		}
		analyzers = parsedAnalyzers
	}
	//As we are downloading the bundle and not running the collectors, notRedactedData field
	//in AnalyzeLocal will be empty (nil)
	return AnalyzeLocal(tmpDir, analyzers, nil)
}

func downloadTroubleshootBundle(bundleURL string, destDir string) error {
	if bundleURL[0] == os.PathSeparator {
		f, err := os.Open(bundleURL)
		if err != nil {
			return errors.Wrap(err, "failed to open support bundle")
		}
		defer f.Close()
		return ExtractTroubleshootBundle(f, destDir)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get workdir")
	}

	tmpDir, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp dir")
	}
	defer os.RemoveAll(tmpDir)

	dst := filepath.Join(tmpDir, "support_bundle.tar.gz")
	err = getter.GetFile(dst, bundleURL, func(c *getter.Client) error {
		c.Pwd = pwd
		c.Decompressors = map[string]getter.Decompressor{}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to read support bundle file")
	}

	f, err := os.Open(dst)
	if err != nil {
		return errors.Wrap(err, "failed to open support bundle")
	}
	defer f.Close()

	return ExtractTroubleshootBundle(f, destDir)
}

func ExtractTroubleshootBundle(reader io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read header from tar")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			name := filepath.Join(destDir, header.Name)
			if err := os.MkdirAll(name, os.FileMode(header.Mode)); err != nil {
				return errors.Wrap(err, "failed to mkdir")
			}
		case tar.TypeReg:
			name := filepath.Join(destDir, header.Name)
			file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				return errors.Wrap(err, "failed to open tar file")
			}
			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return errors.Wrap(err, "failed to extract file")
			}
		}
	}

	return nil
}

func parseAnalyzers(spec string) ([]*troubleshootv1beta2.Analyze, error) {
	troubleshootscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	convertedSpec, err := docrewrite.ConvertToV1Beta2([]byte(spec))
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := decode(convertedSpec, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode analyzers")
	}

	analyzer := obj.(*troubleshootv1beta2.Analyzer)
	return analyzer.Spec.Analyzers, nil
}

func getDefaultAnalyzers() ([]*troubleshootv1beta2.Analyze, error) {
	spec := `apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: defaultAnalyzers
spec:
  analyzers:
    - clusterVersion:
        outcomes:
          - fail:
              when: "< 1.13.0"
              message: The application requires at Kubernetes 1.13.0 or later, and recommends 1.15.0.
              uri: https://www.kubernetes.io
          - warn:
              when: "< 1.15.0"
              message: Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.15.0 or later.
              uri: https://kubernetes.io
          - pass:
              when: ">= 1.15.0"
              message: Your cluster meets the recommended and required versions of Kubernetes.`

	return parseAnalyzers(spec)
}

func (f fileContentProvider) getFileContents(fileName string) ([]byte, error) {
	var file []byte
	var err error
	ok := false
	//First we check if there is any unredacted data matching the fileName.
	if f.collectedData != nil {
		file, ok = f.collectedData[fileName]
	}
	if !ok {
		file, err = ioutil.ReadFile(filepath.Join(f.rootDir, fileName))
		if err != nil {
			return nil, err
		}
	}
	return file, nil
}

func (f fileContentProvider) getChildFileContents(dirName string) (map[string][]byte, error) {
	//First we check if there is any unredacted data matching the pattern.
	matching := make(map[string][]byte)
	if f.collectedData != nil {
		for k, v := range f.collectedData {
			if strings.HasPrefix(k, dirName) {
				matching[k] = v
			}
		}

		for k, v := range f.collectedData {
			if ok, _ := filepath.Match(dirName, k); ok {
				matching[k] = v
			}
		}
	}
	if matching != nil {
		return matching, nil
	}
	files, err := filepath.Glob(filepath.Join(f.rootDir, dirName))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid glob %q", dirName)
	}
	fileArr := map[string][]byte{}
	for _, filePath := range files {
		bytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "read %q", filePath)
		}
		fileArr[filePath] = bytes
	}
	return fileArr, nil
}

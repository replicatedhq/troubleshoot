package analyzer

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	getter "github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	troubleshootscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"k8s.io/client-go/kubernetes/scheme"
)

type fileContentProvider struct {
	rootDir string
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

	analyzers := []*troubleshootv1beta1.Analyze{}

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

	fcp := fileContentProvider{rootDir: tmpDir}

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

func downloadTroubleshootBundle(bundleURL string, destDir string) error {
	if bundleURL[0] == os.PathSeparator {
		f, err := os.Open(bundleURL)
		if err != nil {
			return errors.Wrap(err, "failed to open support bundle")
		}
		defer f.Close()
		return extractTroubleshootBundle(f, destDir)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get workdir")
	}

	tmpDir, err := ioutil.TempDir("", "getter")
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

	return extractTroubleshootBundle(f, destDir)
}

func extractTroubleshootBundle(reader io.Reader, destDir string) error {
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

func parseAnalyzers(spec string) ([]*troubleshootv1beta1.Analyze, error) {
	troubleshootscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode([]byte(spec), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode analyzers")
	}

	analyzer := obj.(*troubleshootv1beta1.Analyzer)
	return analyzer.Spec.Analyzers, nil
}

func getDefaultAnalyzers() ([]*troubleshootv1beta1.Analyze, error) {
	spec := `apiVersion: troubleshoot.replicated.com/v1beta1
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
	return ioutil.ReadFile(filepath.Join(f.rootDir, fileName))
}

func (f fileContentProvider) getChildFileContents(dirName string) (map[string][]byte, error) {
	// TODO: walk sub-dirs
	// return nil, errors.New("not implemented")
	return map[string][]byte{}, nil
}

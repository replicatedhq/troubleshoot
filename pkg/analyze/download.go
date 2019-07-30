package analyzer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	getter "github.com/hashicorp/go-getter"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"gopkg.in/yaml.v2"
)

type fileContentProvider struct {
	rootDir string
}

func DownloadAndAnalyze(ctx context.Context, bundleURL string) ([]*AnalyzeResult, error) {
	tmpDir, err := ioutil.TempDir("", "troubleshoot-k8s")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := downLoadTroubleshootBundle(bundleURL, tmpDir); err != nil {
		return nil, err
	}

	analyzers, err := getTroubleshootAnalyzers()
	if err != nil {
		return nil, err
	}

	fcp := fileContentProvider{rootDir: tmpDir}

	analyzeResults := []*AnalyzeResult{}
	for _, analyzer := range analyzers {
		analyzeResult, err := Analyze(analyzer, fcp.getFileContents, fcp.getChildFileContents)
		if err != nil {
			logger.Printf("an analyzer failed to run: %v\n", err)
			continue
		}

		analyzeResults = append(analyzeResults, analyzeResult)
	}

	return analyzeResults, nil
}

func downLoadTroubleshootBundle(bundleURL, destDir string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "getter")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	dst := filepath.Join(tmpDir, "support_bundle.tar.gz")
	err = getter.GetFile(dst, bundleURL, func(c *getter.Client) error {
		c.Pwd = pwd
		c.Decompressors = map[string]getter.Decompressor{}
		return nil
	})
	if err != nil {
		return err
	}

	f, err := os.Open(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	return extractTroubleshootBundle(f, destDir)
}

func extractTroubleshootBundle(reader io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			name := filepath.Join(destDir, header.Name)
			if err := os.MkdirAll(name, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			name := filepath.Join(destDir, header.Name)
			file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getTroubleshootAnalyzers() ([]*troubleshootv1beta1.Analyze, error) {
	specURL := `https://gist.githubusercontent.com/divolgin/92b512ad4697c7255f383a7c1b56fd83/raw/troubleshoot_v1beta1_preflight.yaml`
	resp, err := http.Get(specURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not download analyzer spec, status code: %v", resp.StatusCode)
	}

	spec, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	preflight := troubleshootv1beta1.Preflight{}
	if err := yaml.Unmarshal([]byte(spec), &preflight); err != nil {
		return nil, err
	}

	return preflight.Spec.Analyzers, nil
}

func (f fileContentProvider) getFileContents(fileName string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(f.rootDir, fileName))
}

func (f fileContentProvider) getChildFileContents(dirName string) (map[string][]byte, error) {
	// TODO: walk sub-dirs
	// return nil, errors.New("not implemnted")
	return map[string][]byte{}, nil
}

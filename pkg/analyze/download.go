package analyzer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	getter "github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

type fileContentProvider struct {
	rootDir string
}

// Analyze local will analyze a locally available (already downloaded) bundle
func AnalyzeLocal(
	ctx context.Context,
	localBundlePath string,
	analyzers []*troubleshootv1beta2.Analyze,
	hostAnalyzers []*troubleshootv1beta2.HostAnalyze,
) ([]*AnalyzeResult, error) {
	rootDir, err := FindBundleRootDir(localBundlePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find root dir")
	}

	fcp := fileContentProvider{rootDir: rootDir}

	analyzeResults := []*AnalyzeResult{}
	for _, analyzer := range analyzers {
		analyzeResult, err := Analyze(ctx, analyzer, fcp.getFileContents, fcp.getChildFileContents)
		if err != nil {
			klog.Errorf("An analyzer failed to run: %v", err)
			continue
		}

		// Filter nil results to prevent panic
		for _, r := range analyzeResult {
			if r != nil {
				analyzeResults = append(analyzeResults, r)
			}
		}
	}

	for _, hostAnalyzer := range hostAnalyzers {
		analyzeResult := HostAnalyze(ctx, hostAnalyzer, fcp.getFileContents, fcp.getChildFileContents)
		analyzeResults = append(analyzeResults, analyzeResult...)
	}

	return analyzeResults, nil
}

func DownloadAndAnalyze(bundleURL string, analyzersSpec string) ([]*AnalyzeResult, error) {
	tmpDir, rootDir, err := DownloadAndExtractSupportBundle(bundleURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find root dir")
	}
	defer os.RemoveAll(tmpDir)

	var analyzers []*troubleshootv1beta2.Analyze
	hostAnalyzers := []*troubleshootv1beta2.HostAnalyze{}

	if analyzersSpec == "" {
		defaultAnalyzers, _, err := getDefaultAnalyzers()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get default analyzers")
		}
		analyzers = defaultAnalyzers
	} else {
		parsedAnalyzers, parsedHostAnalyzers, err := parseAnalyzers(analyzersSpec)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse analyzers")
		}
		analyzers = parsedAnalyzers
		hostAnalyzers = parsedHostAnalyzers
	}

	return AnalyzeLocal(context.Background(), rootDir, analyzers, hostAnalyzers)
}

func DownloadAndExtractSupportBundle(bundleURL string) (string, string, error) {
	// Windows-only: Use working directory to avoid antivirus file locking
	// Linux/macOS: Use system temp (original behavior)
	var tempDir string
	if runtime.GOOS == "windows" {
		cwd, err := os.Getwd()
		if err != nil {
			tempDir = "" // Fallback to system temp
		} else {
			tempDir = cwd
		}
	} else {
		tempDir = "" // Linux/macOS: system temp (unchanged)
	}

	tmpDir, err := os.MkdirTemp(tempDir, "troubleshoot-k8s-")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create temp dir")
	}

	if err := downloadTroubleshootBundle(bundleURL, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", errors.Wrap(err, "failed to download bundle")
	}

	bundleDir, err := FindBundleRootDir(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", errors.Wrap(err, "failed to find root dir")
	}

	_, err = os.Stat(filepath.Join(bundleDir, constants.VERSION_FILENAME))
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", errors.Wrap(err, "failed to read "+constants.VERSION_FILENAME)
	}

	return tmpDir, bundleDir, nil
}

func downloadTroubleshootBundle(bundleURL string, destDir string) error {
	// TODO: Move to separate package support bundle utils package
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

	// Windows-only: Use working directory to avoid antivirus file locking
	// Linux/macOS: Use system temp (original behavior)
	var tempDir string
	if runtime.GOOS == "windows" {
		tempDir = pwd // Use working directory for Windows
	} else {
		tempDir = "" // Linux/macOS: system temp (unchanged)
	}

	tmpDir, err := os.MkdirTemp(tempDir, "troubleshoot-")
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
	// TODO: Move to separate package e.g support bundle package, or sbutils
	// if there are cyclic dependencies
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
			destFileName := filepath.Join(destDir, header.Name)

			dirName := filepath.Dir(destFileName)
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return errors.Wrapf(err, "failed to mkdir for file %s", header.Name)
			}

			file, err := os.OpenFile(destFileName, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				return errors.Wrap(err, "failed to open tar file")
			}
			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return errors.Wrap(err, "failed to extract file")
			}
		case tar.TypeSymlink:
			destFileName := filepath.Join(destDir, header.Name)

			dirName := filepath.Dir(destFileName)
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return errors.Wrapf(err, "failed to mkdir for symlink %s", header.Name)
			}

			// Symlink targets should be absolute paths after extraction
			// for other parts of the code to work correctly e.g redaction, CollectorResult
			targetPath := filepath.Join(filepath.Dir(destFileName), header.Linkname)
			err = os.Symlink(targetPath, destFileName)
			if err != nil {
				return errors.Wrap(err, "failed to create symlink")
			}
		}
	}

	return nil
}

func parseAnalyzers(spec string) ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze, error) {
	troubleshootscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	convertedSpec, err := docrewrite.ConvertToV1Beta2([]byte(spec))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, gvk, err := decode(convertedSpec, nil, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to decode analyzers")
	}

	// SupportBundle overwrites Analyzer if defined
	if gvk.Group == "troubleshoot.sh" && gvk.Version == "v1beta2" && gvk.Kind == "SupportBundle" {
		supportBundle := obj.(*troubleshootv1beta2.SupportBundle)
		return supportBundle.Spec.Analyzers, supportBundle.Spec.HostAnalyzers, nil
	} else if gvk.Group == "troubleshoot.sh" && gvk.Version == "v1beta2" && gvk.Kind == "Analyzer" {
		analyzer := obj.(*troubleshootv1beta2.Analyzer)
		return analyzer.Spec.Analyzers, analyzer.Spec.HostAnalyzers, nil
	}

	return nil, nil, errors.Errorf("invalid gvk %q", gvk)
}

func getDefaultAnalyzers() ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze, error) {
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

// FindBundleRootDir detects whether the bundle is stored inside a subdirectory or not.
// returns the subdirectory path if so, otherwise, returns the path unchanged
func FindBundleRootDir(localBundlePath string) (string, error) {
	f, err := os.Open(localBundlePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open bundle dir")
	}
	defer f.Close()

	names, err := f.Readdirnames(0)
	if err != nil {
		return "", errors.Wrap(err, "failed to read dirnames")
	}

	if len(names) == 0 {
		return "", errors.New("bundle directory is empty")
	}

	isInSubDir := true
	for _, name := range names {
		if name == constants.VERSION_FILENAME {
			isInSubDir = false
			break
		}
	}

	if isInSubDir {
		return filepath.Join(localBundlePath, names[0]), nil
	}

	return localBundlePath, nil
}

func (f fileContentProvider) getFileContents(fileName string) ([]byte, error) {
	contents, err := os.ReadFile(filepath.Join(f.rootDir, fileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &types.NotFoundError{Name: fileName}
		}
		return nil, err
	}
	return contents, nil
}

func excludeFilePaths(files, excludeFiles []string) []string {
	mapExcludeFiles := make(map[string]struct{}, len(excludeFiles))

	for _, path := range excludeFiles {
		mapExcludeFiles[path] = struct{}{}
	}

	var nonexcludedFiles []string

	for _, path := range files {
		if _, found := mapExcludeFiles[path]; !found {
			nonexcludedFiles = append(nonexcludedFiles, path)
		}
	}

	return nonexcludedFiles
}

func (f fileContentProvider) getChildFileContents(dirName string, excludeFiles []string) (map[string][]byte, error) {
	files, err := filepath.Glob(filepath.Join(f.rootDir, dirName))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid glob %q", dirName)
	}

	if len(excludeFiles) > 0 {
		excludeFileNames := []string{}
		for _, excludeFile := range excludeFiles {
			excludeFileName, err := filepath.Glob(filepath.Join(f.rootDir, excludeFile))
			if err != nil {
				return nil, errors.Wrapf(err, "invalid glob %q", excludeFile)
			}
			excludeFileNames = append(excludeFileNames, excludeFileName...)
		}
		files = excludeFilePaths(files, excludeFileNames)
	}

	fileArr := map[string][]byte{}
	for _, filePath := range files {
		bytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "read %q", filePath)
		}
		fileArr[filePath] = bytes
	}
	return fileArr, nil
}

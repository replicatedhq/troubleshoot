package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO (dan): This is VERY similar to the Preflight collect package and should be refactored.
func runCollectors(collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, filename string, bundlePath string, opts SupportBundleCreateOpts) error {

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	var cleanedCollectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       opts.Redact,
			Collect:      desiredCollector,
			ClientConfig: opts.KubernetesRestConfig,
			Namespace:    opts.Namespace,
		}
		cleanedCollectors = append(cleanedCollectors, &collector)
	}

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate kuberentes client")
	}

	if err := cleanedCollectors.CheckRBAC(context.Background()); err != nil {
		return errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range cleanedCollectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return errors.New("insufficient permissions to run all collectors")
	}

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.SinceTime != nil {
		applyLogSinceTime(*opts.SinceTime, &cleanedCollectors)
	}

	// Run preflights collectors synchronously
	for _, collector := range cleanedCollectors {
		if len(collector.RBACErrors) > 0 {
			// don't skip clusterResources collector due to RBAC issues
			if collector.Collect.ClusterResources == nil {
				msg := fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.GetDisplayName())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
				continue
			}
		}

		opts.CollectorProgressCallback(opts.ProgressChan, collector.GetDisplayName())

		result, err := collector.RunCollectorSync(opts.KubernetesRestConfig, k8sClient, globalRedactors)
		if err != nil {
			opts.ProgressChan <- fmt.Errorf("failed to run collector %q: %v", collector.GetDisplayName(), err)
			continue
		}

		if result != nil {
			// results already contain the bundle dir name in their paths
			err = saveCollectorOutput(result, bundlePath, collector)
			if err != nil {
				opts.ProgressChan <- fmt.Errorf("failed to parse collector spec %q: %v", collector.GetDisplayName(), err)
				continue
			}
		}
	}

	return nil
}

func findFileName(basename, extension string) (string, error) {
	n := 1
	name := basename
	for {
		filename := name + "." + extension
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		} else if err != nil {
			return "", errors.Wrap(err, "check file exists")
		}

		name = fmt.Sprintf("%s (%d)", basename, n)
		n = n + 1
	}
}

func ensureCollectorInList(list []*troubleshootv1beta2.Collect, collector troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	for _, inList := range list {
		if collector.ClusterResources != nil && inList.ClusterResources != nil {
			return list
		}
		if collector.ClusterInfo != nil && inList.ClusterInfo != nil {
			return list
		}
	}

	return append(list, &collector)
}

const VersionFilename = "version.yaml"

func writeVersionFile(path string) error {
	version := troubleshootv1beta2.SupportBundleVersion{
		ApiVersion: "troubleshoot.sh/v1beta2",
		Kind:       "SupportBundle",
		Spec: troubleshootv1beta2.SupportBundleVersionSpec{
			VersionNumber: version.Version(),
		},
	}
	b, err := yaml.Marshal(version)
	if err != nil {
		return err
	}

	filename := filepath.Join(path, VersionFilename)
	err = ioutil.WriteFile(filename, b, 0644)
	if err != nil {
		return err
	}

	return nil
}

func applyLogSinceTime(sinceTime time.Time, collectors *collect.Collectors) {

	for _, collector := range *collectors {
		if collector.Collect.Logs != nil {
			if collector.Collect.Logs.Limits == nil {
				collector.Collect.Logs.Limits = new(troubleshootv1beta2.LogLimits)
			}
			collector.Collect.Logs.Limits.SinceTime = metav1.NewTime(sinceTime)
		}
	}
}

func saveCollectorOutput(output map[string][]byte, bundlePath string, c *collect.Collector) error {
	for filename, maybeContents := range output {
		if c.Collect.Copy != nil {
			err := untarAndSave(maybeContents, filepath.Join(bundlePath, filepath.Dir(filename)))
			if err != nil {
				return errors.Wrap(err, "extract copied files")
			}
			continue
		}
		fileDir, fileName := filepath.Split(filename)
		outPath := filepath.Join(bundlePath, fileDir)

		if err := os.MkdirAll(outPath, 0777); err != nil {
			return errors.Wrap(err, "create output file")
		}

		if err := writeFile(filepath.Join(outPath, fileName), maybeContents); err != nil {
			return errors.Wrap(err, "write collector output")
		}
	}

	return nil
}

func untarAndSave(tarFile []byte, bundlePath string) error {
	keys := make([]string, 0)
	dirs := make(map[string]*tar.Header)
	files := make(map[string][]byte)
	fileHeaders := make(map[string]*tar.Header)
	tarReader := tar.NewReader(bytes.NewBuffer(tarFile))
	//Extract and separate tar contents in file and folders, keeping header info from each one.
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		switch header.Typeflag {
		case tar.TypeDir:
			dirs[header.Name] = header
		case tar.TypeReg:
			file := new(bytes.Buffer)
			_, err = io.Copy(file, tarReader)
			if err != nil {
				return err
			}
			files[header.Name] = file.Bytes()
			fileHeaders[header.Name] = header
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", header.Name, header.FileInfo().Mode())
		}
	}
	//Create directories from base path: <namespace>/<pod name>/containerPath
	if err := os.MkdirAll(filepath.Join(bundlePath), 0777); err != nil {
		return errors.Wrap(err, "create output file")
	}
	//Order folders stored in variable keys to start always by parent folder. That way folder info is preserved.
	for k := range dirs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	//Orderly create folders.
	for _, k := range keys {
		if err := os.Mkdir(filepath.Join(bundlePath, k), dirs[k].FileInfo().Mode().Perm()); err != nil {
			return errors.Wrap(err, "create output file")
		}
	}
	//Populate folders with respective files and its permissions stored in the header.
	for k, v := range files {
		if err := ioutil.WriteFile(filepath.Join(bundlePath, k), v, fileHeaders[k].FileInfo().Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(filename string, contents []byte) error {
	if err := ioutil.WriteFile(filename, contents, 0644); err != nil {
		return err
	}

	return nil
}

func tarSupportBundleDir(inputDir, outputFilename string) error {
	fileWriter, err := os.Create(outputFilename)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer fileWriter.Close()

	gzipWriter := gzip.NewWriter(fileWriter)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err = filepath.Walk(inputDir, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fileMode := info.Mode()
		if !fileMode.IsRegular() { // support bundle can have only files
			return nil
		}

		parentDirName := filepath.Dir(inputDir) // this is to have the files inside a subdirectory
		nameInArchive, err := filepath.Rel(parentDirName, filename)
		if err != nil {
			return errors.Wrap(err, "failed to create relative file name")
		}

		// tar.FileInfoHeader call causes a crash in static builds
		// https://github.com/golang/go/issues/24787
		hdr := &tar.Header{
			Name:     nameInArchive,
			ModTime:  info.ModTime(),
			Mode:     int64(fileMode.Perm()),
			Typeflag: tar.TypeReg,
			Size:     info.Size(),
		}

		err = tarWriter.WriteHeader(hdr)
		if err != nil {
			return errors.Wrap(err, "failed to write tar header")
		}

		err = func() error {
			fileReader, err := os.Open(filename)
			if err != nil {
				return errors.Wrap(err, "failed to open source file")
			}
			defer fileReader.Close()

			_, err = io.Copy(tarWriter, fileReader)
			if err != nil {
				return errors.Wrap(err, "failed to copy file into archive")
			}

			return nil
		}()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk source dir")
	}

	return nil
}

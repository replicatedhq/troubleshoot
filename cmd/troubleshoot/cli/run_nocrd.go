package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/viper"
	"github.com/tj/go-spin"
	"gopkg.in/yaml.v2"
)

func runTroubleshootNoCRD(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

	collectorContent := ""
	if !isURL(arg) {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return fmt.Errorf("%s was not found", arg)
		}

		b, err := ioutil.ReadFile(arg)
		if err != nil {
			return errors.Wrap(err, "read spec file")
		}

		collectorContent = string(b)
	} else {
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return errors.Wrap(err, "make request")
		}
		req.Header.Set("User-Agent", "Replicated_Troubleshoot/v1beta1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return errors.Wrap(err, "execute request")
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "read responce body")
		}

		collectorContent = string(body)
	}

	collector := troubleshootv1beta1.Collector{}
	if err := yaml.Unmarshal([]byte(collectorContent), &collector); err != nil {
		return fmt.Errorf("unable to parse %s collectors", arg)
	}

	s := spin.New()
	finishedCh := make(chan bool, 1)
	progressChan := make(chan interface{}, 1)
	go func() {
		currentDir := ""
		for {
			select {
			case msg := <-progressChan:
				switch msg := msg.(type) {
				case error:
					c := color.New(color.FgHiRed)
					c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
				case string:
					currentDir = filepath.Base(msg)
				}
			case <-finishedCh:
				fmt.Printf("\r%s\r", cursor.ClearEntireLine())
				return
			case <-time.After(time.Millisecond * 100):
				if currentDir == "" {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
				} else {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s %s", cursor.ClearEntireLine(), s.Next(), currentDir)
				}
			}
		}
	}()
	defer func() {
		finishedCh <- true
	}()

	archivePath, err := runCollectors(v, collector, progressChan)
	if err != nil {
		return errors.Wrap(err, "run collectors")
	}

	fmt.Printf("\r%s\r", cursor.ClearEntireLine())

	if len(collector.Spec.AfterCollection) == 0 {
		msg := archivePath
		if appName := collector.Labels["applicationName"]; appName != "" {
			f := `A support bundle for %s has been created in this directory
named %s. Please upload it on the Troubleshoot page of
the %s Admin Console to begin analysis.`
			msg = fmt.Sprintf(f, appName, archivePath, appName)
		}

		fmt.Printf("%s\n", msg)

		return nil
	}

	for _, ac := range collector.Spec.AfterCollection {
		if ac.UploadResultsTo != nil {
			if err := uploadSupportBundle(ac.UploadResultsTo, archivePath); err != nil {
				return errors.Wrap(err, "upload support bundle")
			}
		} else if ac.Callback != nil {
			if err := callbackSupportBundleAPI(ac.Callback, archivePath); err != nil {
				return errors.Wrap(err, "execute callback")
			}
		}
	}

	fmt.Printf("\nA support bundle has been created in the current directory named %q\n", archivePath)
	return nil
}

func runCollectors(v *viper.Viper, collector troubleshootv1beta1.Collector, progressChan chan interface{}) (string, error) {
	bundlePath, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return "", errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(bundlePath)

	versionFilename, err := writeVersionFile(bundlePath)
	if err != nil {
		return "", errors.Wrap(err, "write version file")
	}

	desiredCollectors := make([]*troubleshootv1beta1.Collect, 0, 0)
	for _, definedCollector := range collector.Spec.Collectors {
		desiredCollectors = append(desiredCollectors, definedCollector)
	}
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	collectorDirs := []string{}

	config, err := KubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	// Run preflights collectors synchronously
	for _, desiredCollector := range desiredCollectors {
		collector := collect.Collector{
			Redact:       true,
			Collect:      desiredCollector,
			ClientConfig: config,
		}

		result, err := collector.RunCollectorSync()
		if err != nil {
			progressChan <- fmt.Errorf("failed to run collector %q: %v", collector.GetDisplayName(), err)
			continue
		}

		collectorDir, err := parseAndSaveCollectorOutput(string(result), bundlePath)
		if err != nil {
			progressChan <- fmt.Errorf("failed to parse collector spec %q: %v", collector.GetDisplayName(), err)
			continue
		}

		if collectorDir == "" {
			continue
		}

		progressChan <- collectorDir
		collectorDirs = append(collectorDirs, collectorDir)
	}

	tarGz := archiver.TarGz{
		Tar: &archiver.Tar{
			ImplicitTopLevelFolder: false,
		},
	}

	// version file should be first in tar archive for quick extraction
	paths := []string{
		versionFilename,
	}
	for _, collectorDir := range collectorDirs {
		paths = append(paths, collectorDir)
	}

	filename, err := findFileName("support-bundle", "tar.gz")
	if err != nil {
		return "", errors.Wrap(err, "find file name")
	}

	if err := tarGz.Archive(paths, filename); err != nil {
		return "", errors.Wrap(err, "create archive")
	}

	return filename, nil
}

func parseAndSaveCollectorOutput(output string, bundlePath string) (string, error) {
	dir := ""

	input := make(map[string]interface{})
	if err := json.Unmarshal([]byte(output), &input); err != nil {
		return "", errors.Wrap(err, "unmarshal output")
	}

	for filename, maybeContents := range input {
		fileDir, fileName := filepath.Split(filename)
		outPath := filepath.Join(bundlePath, fileDir)
		dir = outPath

		if err := os.MkdirAll(outPath, 0777); err != nil {
			return "", errors.Wrap(err, "create output file")
		}

		switch maybeContents.(type) {
		case string:
			decoded, err := base64.StdEncoding.DecodeString(maybeContents.(string))
			if err != nil {
				return "", errors.Wrap(err, "decode collector output")
			}

			if err := writeFile(filepath.Join(outPath, fileName), decoded); err != nil {
				return "", errors.Wrap(err, "write collector output")
			}

		case map[string]interface{}:
			for k, v := range maybeContents.(map[string]interface{}) {
				s, _ := filepath.Split(filepath.Join(outPath, fileName, k))
				if err := os.MkdirAll(s, 0777); err != nil {
					return "", errors.Wrap(err, "write output directories")
				}

				decoded, err := base64.StdEncoding.DecodeString(v.(string))
				if err != nil {
					return "", errors.Wrap(err, "decode output")
				}
				if err := writeFile(filepath.Join(outPath, fileName, k), decoded); err != nil {
					return "", errors.Wrap(err, "write output")
				}
			}
		}
	}

	return dir, nil
}

func uploadSupportBundle(r *troubleshootv1beta1.ResultRequest, archivePath string) error {
	contentType := getExpectedContentType(r.URI)
	if contentType != "" && contentType != "application/tar+gzip" {
		return fmt.Errorf("cannot upload content type %s", contentType)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return errors.Wrap(err, "open file")
	}
	defer f.Close()

	fileStat, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "stat file")
	}

	req, err := http.NewRequest(r.Method, r.URI, f)
	if err != nil {
		return errors.Wrap(err, "create request")
	}
	req.ContentLength = fileStat.Size()
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func getExpectedContentType(uploadURL string) string {
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("Content-Type")
}

func callbackSupportBundleAPI(r *troubleshootv1beta1.ResultRequest, archivePath string) error {
	req, err := http.NewRequest(r.Method, r.URI, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

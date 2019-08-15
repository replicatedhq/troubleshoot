package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmetalpbalkan/go-cursor"
	"github.com/mholt/archiver"
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
			return err
		}

		collectorContent = string(b)
	} else {
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Replicated_Troubleshoot/v1beta1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		collectorContent = string(body)
	}

	collector := troubleshootv1beta1.Collector{}
	if err := yaml.Unmarshal([]byte(collectorContent), &collector); err != nil {
		return fmt.Errorf("unable to parse %s collectors", arg)
	}

	s := spin.New()
	finishedCh := make(chan bool, 1)
	progressChan := make(chan string, 1)
	go func() {
		currentDir := ""
		for {
			select {
			case dir := <-progressChan:
				currentDir = filepath.Base(dir)
			case <-finishedCh:
				fmt.Printf("\r")
				return
			case <-time.After(time.Millisecond * 100):
				if currentDir == "" {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
				} else {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s: %s", cursor.ClearEntireLine(), s.Next(), currentDir)
				}
			}
		}
	}()
	defer func() {
		finishedCh <- true
	}()

	archivePath, err := runCollectors(v, collector, progressChan)
	if err != nil {
		return err
	}

	fmt.Printf("\r%s", cursor.ClearEntireLine())

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

func runCollectors(v *viper.Viper, collector troubleshootv1beta1.Collector, progressChan chan string) (string, error) {
	bundlePath, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(bundlePath)

	versionFilename, err := writeVersionFile(bundlePath)
	if err != nil {
		return "", err
	}

	desiredCollectors := make([]*troubleshootv1beta1.Collect, 0, 0)
	for _, definedCollector := range collector.Spec {
		desiredCollectors = append(desiredCollectors, definedCollector)
	}
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	collectorDirs := []string{}

	// Run preflights collectors synchronously
	for _, desiredCollector := range desiredCollectors {
		collector := collect.Collector{
			Redact:  true,
			Collect: desiredCollector,
		}

		result, err := collector.RunCollectorSync()
		if err != nil {
			progressChan <- fmt.Sprintf("failed to run collector %v", collector)
			continue
		}

		collectorDir, err := parseAndSaveCollectorOutput(string(result), bundlePath)
		if err != nil {
			progressChan <- fmt.Sprintf("failed to parse collector spec: %v", collector)
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

	if err := tarGz.Archive(paths, "support-bundle.tar.gz"); err != nil {
		return "", err
	}

	return "support-bundle.tar.gz", nil
}

func parseAndSaveCollectorOutput(output string, bundlePath string) (string, error) {
	dir := ""

	input := make(map[string]interface{})
	if err := json.Unmarshal([]byte(output), &input); err != nil {
		return "", err
	}

	for filename, maybeContents := range input {
		fileDir, fileName := filepath.Split(filename)
		outPath := filepath.Join(bundlePath, fileDir)
		dir = outPath

		if err := os.MkdirAll(outPath, 0777); err != nil {
			return "", err
		}

		switch maybeContents.(type) {
		case string:
			decoded, err := base64.StdEncoding.DecodeString(maybeContents.(string))
			if err != nil {
				return "", err
			}

			if err := writeFile(filepath.Join(outPath, fileName), decoded); err != nil {
				return "", err
			}

		case map[string]interface{}:
			for k, v := range maybeContents.(map[string]interface{}) {
				s, _ := filepath.Split(filepath.Join(outPath, fileName, k))
				if err := os.MkdirAll(s, 0777); err != nil {
					return "", err
				}

				decoded, err := base64.StdEncoding.DecodeString(v.(string))
				if err != nil {
					return "", err
				}
				if err := writeFile(filepath.Join(outPath, fileName, k), decoded); err != nil {
					return "", err
				}
			}
		}
	}

	return dir, nil
}

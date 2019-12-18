package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmetalpbalkan/go-cursor"
	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/viper"
	"github.com/tj/go-spin"
	"gopkg.in/yaml.v2"
)

func runPreflights(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

	preflightContent := ""
	if !isURL(arg) {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return fmt.Errorf("%s was not found", arg)
		}

		b, err := ioutil.ReadFile(arg)
		if err != nil {
			return err
		}

		preflightContent = string(b)
	} else {
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Replicated_Preflight/v1beta1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		preflightContent = string(body)
	}

	preflight := troubleshootv1beta1.Preflight{}
	if err := yaml.Unmarshal([]byte(preflightContent), &preflight); err != nil {
		return fmt.Errorf("unable to parse %s as a preflight", arg)
	}

	s := spin.New()
	finishedCh := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-finishedCh:
				fmt.Printf("\r")
				return
			case <-time.After(time.Millisecond * 100):
				fmt.Printf("\r  \033[36mRunning Preflight checks\033[m %s ", s.Next())
			}
		}
	}()
	defer func() {
		finishedCh <- true
	}()

	allCollectedData, err := runCollectors(v, preflight)
	if err != nil {
		return err
	}

	getCollectedFileContents := func(fileName string) ([]byte, error) {
		contents, ok := allCollectedData[fileName]
		if !ok {
			return nil, fmt.Errorf("file %s was not collected", fileName)
		}

		return contents, nil
	}
	getChildCollectedFileContents := func(prefix string) (map[string][]byte, error) {
		matching := make(map[string][]byte)
		for k, v := range allCollectedData {
			if strings.HasPrefix(k, prefix) {
				matching[k] = v
			}
		}

		return matching, nil
	}

	analyzeResults := []*analyzerunner.AnalyzeResult{}
	for _, analyzer := range preflight.Spec.Analyzers {
		analyzeResult, err := analyzerunner.Analyze(analyzer, getCollectedFileContents, getChildCollectedFileContents)
		if err != nil {
			logger.Printf("an analyzer failed to run: %v\n", err)
			continue
		}

		if analyzeResult != nil {
			analyzeResults = append(analyzeResults, analyzeResult)
		}
	}

	finishedCh <- true

	if preflight.Spec.UploadResultsTo != "" {
		tryUploadResults(preflight.Spec.UploadResultsTo, preflight.Name, analyzeResults)
	}
	if v.GetBool("interactive") {
		return showInteractiveResults(preflight.Name, analyzeResults)
	}

	return showStdoutResults(preflight.Name, analyzeResults)
}

func runCollectors(v *viper.Viper, preflight troubleshootv1beta1.Preflight) (map[string][]byte, error) {
	desiredCollectors := make([]*troubleshootv1beta1.Collect, 0, 0)
	for _, definedCollector := range preflight.Spec.Collectors {
		desiredCollectors = append(desiredCollectors, definedCollector)
	}
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	allCollectedData := make(map[string][]byte)

	config, err := KubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	// Run preflights collectors synchronously
	for _, desiredCollector := range desiredCollectors {
		collector := collect.Collector{
			Redact:       true,
			Collect:      desiredCollector,
			ClientConfig: config,
			Namespace:    v.GetString("namespace"),
		}

		result, err := collector.RunCollectorSync()
		if err != nil {
			return nil, errors.Wrap(err, "failed to run collector")
		}

		if result != nil {
			output, err := parseCollectorOutput(string(result))
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse collector output")
			}
			for k, v := range output {
				allCollectedData[k] = v
			}
		}
	}

	return allCollectedData, nil
}

func parseCollectorOutput(output string) (map[string][]byte, error) {
	input := make(map[string]interface{})
	files := make(map[string][]byte)
	if err := json.Unmarshal([]byte(output), &input); err != nil {
		return nil, err
	}

	for filename, maybeContents := range input {
		fileDir, fileName := filepath.Split(filename)

		switch maybeContents.(type) {
		case string:
			decoded, err := base64.StdEncoding.DecodeString(maybeContents.(string))
			if err != nil {
				return nil, err
			}
			files[filepath.Join(fileDir, fileName)] = decoded

		case map[string]interface{}:
			for k, v := range maybeContents.(map[string]interface{}) {
				decoded, err := base64.StdEncoding.DecodeString(v.(string))
				if err != nil {
					return nil, err
				}
				files[filepath.Join(fileDir, fileName, k)] = decoded
			}
		}
	}

	return files, nil
}

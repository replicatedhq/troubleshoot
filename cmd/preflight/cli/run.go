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

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
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
	if err := json.Unmarshal([]byte(preflightContent), &preflight); err != nil {
		return errors.Wrapf(err, "failed to parse %s as a preflight", arg)
	}

	s := spin.New()
	finishedCh := make(chan bool, 1)
	progressChan := make(chan interface{}, 0) // non-zero buffer will result in missed messages
	go func() {
		for {
			select {
			case msg, ok := <-progressChan:
				if !ok {
					continue
				}
				switch msg := msg.(type) {
				case error:
					c := color.New(color.FgHiRed)
					c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
				case string:
					c := color.New(color.FgCyan)
					c.Println(fmt.Sprintf("%s\r * %s", cursor.ClearEntireLine(), msg))
				}
			case <-time.After(time.Millisecond * 100):
				fmt.Printf("\r  \033[36mRunning Preflight checks\033[m %s ", s.Next())
			case <-finishedCh:
				fmt.Printf("\r%s\r", cursor.ClearEntireLine())
				return
			}
		}
	}()
	defer func() {
		close(finishedCh)
	}()

	allCollectedData, err := runCollectors(v, preflight, progressChan)
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
			analyzeResult = &analyzerunner.AnalyzeResult{
				IsFail:  true,
				Title:   "Analyzer Failed",
				Message: err.Error(),
			}
		}

		if analyzeResult != nil {
			analyzeResults = append(analyzeResults, analyzeResult)
		}
	}

	if preflight.Spec.UploadResultsTo != "" {
		err := uploadResults(preflight.Spec.UploadResultsTo, analyzeResults)
		if err != nil {
			progressChan <- err
		}
	}

	finishedCh <- true

	if v.GetBool("interactive") {
		if len(analyzeResults) == 0 {
			return errors.New("no data has been collected")
		}
		return showInteractiveResults(preflight.Name, analyzeResults)
	}

	return showStdoutResults(v.GetString("format"), preflight.Name, analyzeResults)
}

func runCollectors(v *viper.Viper, preflight troubleshootv1beta1.Preflight, progressChan chan interface{}) (map[string][]byte, error) {
	collectSpecs := make([]*troubleshootv1beta1.Collect, 0, 0)
	collectSpecs = append(collectSpecs, preflight.Spec.Collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	allCollectedData := make(map[string][]byte)

	config, err := KubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	var collectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       true,
			Collect:      desiredCollector,
			ClientConfig: config,
			Namespace:    v.GetString("namespace"),
		}
		collectors = append(collectors, &collector)
	}

	if err := collectors.CheckRBAC(); err != nil {
		return nil, errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range collectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			progressChan <- e
		}
	}

	if foundForbidden && !v.GetBool("collect-without-permissions") {
		if preflight.Spec.UploadResultsTo != "" {
			err := uploadErrors(preflight.Spec.UploadResultsTo, collectors)
			if err != nil {
				progressChan <- err
			}
		}
		return nil, errors.New("insufficient permissions to run all collectors")
	}

	// Run preflights collectors synchronously
	for _, collector := range collectors {
		if len(collector.RBACErrors) > 0 {
			// don't skip clusterResources collector due to RBAC issues
			if collector.Collect.ClusterResources == nil {
				progressChan <- fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.GetDisplayName())
				continue
			}
		}

		result, err := collector.RunCollectorSync()
		if err != nil {
			progressChan <- errors.Errorf("failed to run collector %s: %v\n", collector.GetDisplayName(), err)
			continue
		}

		if result != nil {
			output, err := parseCollectorOutput(string(result))
			if err != nil {
				progressChan <- errors.Errorf("failed to parse collector output %s: %v\n", collector.GetDisplayName(), err)
				continue
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

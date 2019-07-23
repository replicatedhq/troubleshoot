package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	collectrunner "github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func runPreflightsNoCRD(v *viper.Viper, arg string) error {
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
		resp, err := http.Get(arg)
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
			fmt.Printf("an analyzer failed to run: %v\n", err)
			continue
		}

		analyzeResults = append(analyzeResults, analyzeResult)
	}

	if preflight.Spec.UploadResultsTo != "" {
		tryUploadResults(preflight.Spec.UploadResultsTo, preflight.Name, analyzeResults)
	}
	if v.GetBool("interactive") {
		return showInteractiveResults(preflight.Name, analyzeResults)
	}

	fmt.Printf("only interactive results are supported\n")
	return nil
}

func runCollectors(v *viper.Viper, preflight troubleshootv1beta1.Preflight) (map[string][]byte, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	restClient := clientset.CoreV1().RESTClient()

	serviceAccountName := v.GetString("serviceaccount")
	if serviceAccountName == "" {
		generatedServiceAccountName, err := createServiceAccount(preflight, v.GetString("namespace"), clientset)
		if err != nil {
			return nil, err
		}
		defer removeServiceAccount(generatedServiceAccountName, v.GetString("namespace"), clientset)

		serviceAccountName = generatedServiceAccountName
	}

	// deploy an object that "owns" everything to aid in cleanup
	owner := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("preflight-%s-owner", preflight.Name),
			Namespace: v.GetString("namespace"),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		Data: make(map[string]string),
	}
	if err := client.Create(context.Background(), &owner); err != nil {
		return nil, err
	}
	defer func() {
		if err := client.Delete(context.Background(), &owner); err != nil {
			fmt.Println("failed to clean up preflight.")
		}
	}()

	// deploy all collectors
	desiredCollectors := make([]*troubleshootv1beta1.Collect, 0, 0)
	for _, definedCollector := range preflight.Spec.Collectors {
		desiredCollectors = append(desiredCollectors, definedCollector)
	}
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	podsCreated := make([]*corev1.Pod, 0, 0)
	podsDeleted := make([]*corev1.Pod, 0, 0)
	allCollectedData := make(map[string][]byte)

	resyncPeriod := time.Second
	ctx := context.Background()
	watchList := cache.NewListWatchFromClient(restClient, "pods", "", fields.Everything())
	_, controller := cache.NewInformer(watchList, &corev1.Pod{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				newPod, ok := newObj.(*corev1.Pod)
				if !ok {
					return
				}
				oldPod, ok := oldObj.(*corev1.Pod)
				if !ok {
					return
				}
				labels := newPod.Labels
				troubleshootRole, ok := labels["troubleshoot-role"]
				if !ok || troubleshootRole != "preflight" {
					return
				}
				preflightName, ok := labels["preflight"]
				if !ok || preflightName != preflight.Name {
					return
				}

				if oldPod.Status.Phase == newPod.Status.Phase {
					return
				}

				if newPod.Status.Phase == corev1.PodFailed {
					podsDeleted = append(podsDeleted, newPod)
					return
				}

				if newPod.Status.Phase != corev1.PodSucceeded {
					return
				}

				podLogOpts := corev1.PodLogOptions{}

				req := clientset.CoreV1().Pods(newPod.Namespace).GetLogs(newPod.Name, &podLogOpts)
				podLogs, err := req.Stream()
				if err != nil {
					fmt.Println("get stream")
					return
				}
				defer podLogs.Close()

				buf := new(bytes.Buffer)
				_, err = io.Copy(buf, podLogs)
				if err != nil {
					fmt.Println("copy logs")
					return
				}

				collectedData, err := parseCollectorOutput(buf.String())
				if err != nil {
					fmt.Printf("parse collected data: %v\n", err)
					return
				}
				for k, v := range collectedData {
					allCollectedData[k] = v
				}

				if err := client.Delete(context.Background(), newPod); err != nil {
					fmt.Println("delete pod")
				}
				podsDeleted = append(podsDeleted, newPod)
			},
		})
	go func() {
		controller.Run(ctx.Done())
	}()

	s := runtime.NewScheme()
	s.AddKnownTypes(schema.GroupVersion{Group: "", Version: "v1"}, &corev1.ConfigMap{})
	for _, collector := range desiredCollectors {
		_, pod, err := collectrunner.CreateCollector(client, s, &owner, preflight.Name, v.GetString("namespace"), serviceAccountName, "preflight", collector, v.GetString("image"), v.GetString("pullpolicy"))
		if err != nil {
			return nil, err
		}
		podsCreated = append(podsCreated, pod)
	}

	start := time.Now()
	for {
		if start.Add(time.Second * 30).Before(time.Now()) {
			fmt.Println("timeout running preflight")
			return nil, err
		}

		if len(podsDeleted) == len(podsCreated) {
			break
		}

		time.Sleep(time.Millisecond * 200)
	}

	ctx.Done()

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

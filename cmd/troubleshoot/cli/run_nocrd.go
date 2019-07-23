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
	"time"

	"github.com/mholt/archiver"
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

func runTroubleshootNoCRD(v *viper.Viper, arg string) error {
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
		resp, err := http.Get(arg)
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

	archivePath, err := runCollectors(v, collector)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", archivePath)

	return nil
}

func runCollectors(v *viper.Viper, collector troubleshootv1beta1.Collector) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", err
	}

	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return "", err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	restClient := clientset.CoreV1().RESTClient()

	// deploy an object that "owns" everything to aid in cleanup
	owner := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("troubleshoot-%s-owner", collector.Name),
			Namespace: v.GetString("namespace"),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		Data: make(map[string]string),
	}
	if err := client.Create(context.Background(), &owner); err != nil {
		return "", err
	}
	defer func() {
		if err := client.Delete(context.Background(), &owner); err != nil {
			fmt.Println("failed to clean up preflight.")
		}
	}()

	// deploy all collectors
	desiredCollectors := make([]*troubleshootv1beta1.Collect, 0, 0)
	for _, definedCollector := range collector.Spec {
		desiredCollectors = append(desiredCollectors, definedCollector)
	}
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	desiredCollectors = ensureCollectorInList(desiredCollectors, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

	podsCreated := make([]*corev1.Pod, 0, 0)
	podsDeleted := make([]*corev1.Pod, 0, 0)

	collectorDirs := []string{}

	bundlePath, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return "", err
	}
	// defer os.RemoveAll(bundlePath)

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
				if !ok || troubleshootRole != "troubleshoot" {
					return
				}

				collectorName, ok := labels["troubleshoot"]
				if !ok || collectorName != collector.Name {
					return
				}

				if oldPod.Status.Phase == newPod.Status.Phase {
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

				collectorDir, err := parseAndSaveCollectorOutput(buf.String(), bundlePath)
				if err != nil {
					fmt.Printf("parse collected data: %v\n", err)
					return
				}

				// empty dir name will make tar fail
				if collectorDir == "" {
					fmt.Printf("pod %s did not return any files\n", newPod.Name)
					return
				}

				collectorDirs = append(collectorDirs, collectorDir)

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
	for _, collect := range desiredCollectors {
		_, pod, err := collectrunner.CreateCollector(client, s, &owner, collector.Name, v.GetString("namespace"), "troubleshoot", collect, v.GetString("image"), v.GetString("pullpolicy"))
		if err != nil {
			return "", err
		}
		podsCreated = append(podsCreated, pod)
	}

	start := time.Now()
	for {
		if start.Add(time.Second * 30).Before(time.Now()) {
			fmt.Println("timeout running troubleshoot")
			return "", err
		}

		if len(podsDeleted) == len(podsCreated) {
			break
		}

		time.Sleep(time.Millisecond * 200)
	}

	ctx.Done()

	tarGz := archiver.TarGz{
		Tar: &archiver.Tar{
			ImplicitTopLevelFolder: false,
		},
	}

	paths := make([]string, 0, 0)
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

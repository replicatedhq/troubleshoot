package supportbundle

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

func runHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {

	var collectResult map[string][]byte

	if opts.RunHostCollectorsInPod {
		collectResult = runRemoteHostCollectors(ctx, hostCollectors, bundlePath, opts)
	} else {
		collectResult = runLocalHostCollectors(ctx, hostCollectors, bundlePath, opts)
	}

	// redact result if any
	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.Redact {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "Host collectors")
		span.SetAttributes(attribute.String("type", "Redactors"))
		err := collect.RedactResult(bundlePath, collectResult, globalRedactors)
		if err != nil {
			err = errors.Wrap(err, "failed to redact host collector results")
			span.SetStatus(codes.Error, err.Error())
			return collectResult, err
		}
		span.End()
	}

	return collectResult, nil
}

func runCollectors(ctx context.Context, collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {
	var allCollectors []collect.Collector
	var foundForbidden bool

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})
	collectSpecs = collect.DedupCollectors(collectSpecs)
	collectSpecs = collect.EnsureClusterResourcesFirst(collectSpecs)

	opts.KubernetesRestConfig.QPS = constants.DEFAULT_CLIENT_QPS
	opts.KubernetesRestConfig.Burst = constants.DEFAULT_CLIENT_BURST
	opts.KubernetesRestConfig.UserAgent = fmt.Sprintf("%s/%s", constants.DEFAULT_CLIENT_USER_AGENT, version.Version())

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	allCollectorsMap := make(map[reflect.Type][]collect.Collector)
	allCollectedData := make(map[string][]byte)

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, bundlePath, opts.Namespace, opts.KubernetesRestConfig, k8sClient, opts.SinceTime); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(ctx, collector, desiredCollector, opts.KubernetesRestConfig, opts.Namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}
				collectorType := reflect.TypeOf(collector)
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)
			}
		}
	}

	for _, collectors := range allCollectorsMap {
		if mergeCollector, ok := collectors[0].(collect.MergeableCollector); ok {
			mergedCollectors, err := mergeCollector.Merge(collectors)
			if err != nil {
				msg := fmt.Sprintf("failed to merge collector: %s: %s", mergeCollector.Title(), err)
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
			}
			allCollectors = append(allCollectors, mergedCollectors...)
		} else {
			allCollectors = append(allCollectors, collectors...)
		}

		foundForbidden = false
		for _, collector := range collectors {
			for _, e := range collector.GetRBACErrors() {
				foundForbidden = true
				opts.ProgressChan <- e
			}
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return nil, collect.ErrInsufficientPermissionsToRun
	}

	for _, collector := range allCollectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			msg := fmt.Sprintf("excluding %q collector", collector.Title())
			opts.CollectorProgressCallback(opts.ProgressChan, msg)
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*collect.CollectClusterResources); !ok {
				msg := fmt.Sprintf("skipping collector %q with insufficient RBAC permissions", collector.Title())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
				span.SetStatus(codes.Error, "skipping collector, insufficient RBAC permissions")
				span.End()
				continue
			}
		}
		opts.CollectorProgressCallback(opts.ProgressChan, collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}

		for k, v := range result {
			allCollectedData[k] = v
		}
		span.End()
	}

	collectResult := allCollectedData

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.Redact {
		// TODO: Should we record how long each redactor takes?
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "In-cluster collectors")
		span.SetAttributes(attribute.String("type", "Redactors"))
		err := collect.RedactResult(bundlePath, collectResult, globalRedactors)
		if err != nil {
			err := errors.Wrap(err, "failed to redact in cluster collector results")
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return collectResult, err
		}
		span.End()
	}

	return collectResult, nil
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

func getAnalysisFile(analyzeResults []*analyze.AnalyzeResult) (io.Reader, error) {
	data := convert.FromAnalyzerResult(analyzeResults)
	analysis, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analysis")
	}

	return bytes.NewBuffer(analysis), nil
}

func runLocalHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, bundlePath string, opts SupportBundleCreateOpts) map[string][]byte {
	collectSpecs := make([]*troubleshootv1beta2.HostCollect, 0)
	collectSpecs = append(collectSpecs, hostCollectors...)

	allCollectedData := make(map[string][]byte)

	var collectors []collect.HostCollector
	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, bundlePath)
		if ok {
			collectors = append(collectors, collector)
		}
	}

	for _, collector := range collectors {
		// TODO: Add context to host collectors
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			opts.ProgressChan <- fmt.Sprintf("[%s] Excluding host collector", collector.Title())
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		opts.ProgressChan <- fmt.Sprintf("[%s] Running host collector...", collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			opts.ProgressChan <- errors.Errorf("failed to run host collector: %s: %v", collector.Title(), err)
		}
		span.End()
		for k, v := range result {
			allCollectedData[k] = v
		}
	}

	return allCollectedData
}

func getExecOutputs(
	ctx context.Context, clientConfig *rest.Config, client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta2.HostCollect,
) ([]byte, []byte, []string) {
	container := pod.Spec.Containers[0].Name

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, []string{err.Error()}
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"/troubleshoot/collect", "-"},
		Container: container,
		Stdin:     true,
		Stdout:    false,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	stdin := new(bytes.Buffer)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), []string{err.Error()}
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

func runRemoteHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, bundlePath string, opts SupportBundleCreateOpts) map[string][]byte {
	output := collect.NewResult()

	// convert host collectors into a HostCollector spec
	spec := createHostCollectorsSpec(hostCollectors)
	specJSON, err := convertHostCollectorSpecToJSON(spec)
	if err != nil {
		// TODO: error handling
		return nil
	}
	klog.V(2).Infof("HostCollector spec: %s", specJSON)

	clientset, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		// TODO: error handling
		return nil
	}

	// TODO: rbac check

	nodeList, err := getNodeList(clientset, opts)
	if err != nil {
		// TODO: error handling
		return nil
	}
	klog.V(2).Infof("Node list to run remote host collectors: %s", nodeList.Nodes)

	// create remote pod for each node
	labels := map[string]string{
		"troubleshoot.sh/remote-collector": "true",
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	nodeLogs := make(map[string][]byte)

	for _, node := range nodeList.Nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()

			// TODO: set timeout waiting

			// create a remote pod spec to run the host collectors
			nodeSelector := map[string]string{
				"kubernetes.io/hostname": node,
			}
			pod, err := createHostCollectorPod(ctx, clientset, nodeSelector, labels)
			if err != nil {
				// TODO: error handling
				return
			}
			klog.V(2).Infof("Created Remote Host Collector Pod %s", pod.Name)

			// wait for the pod to be running
			// err = waitForPodRunning(ctx, clientset, pod)
			// if err != nil {
			// 	// TODO: error handling
			// 	return
			// }
			time.Sleep(10 * time.Second)

			getExecOutputs()

			// // extract logs from the pod
			// var logs []byte
			// logs, err = getPodLogs(ctx, clientset, pod)
			// if err != nil {
			// 	// TODO: error handling
			// 	return
			// }

			// wait for log stream to catch up
			time.Sleep(1 * time.Second)

			mu.Lock()
			nodeLogs[node] = logs
			mu.Unlock()

		}(node)
	}
	wg.Wait()

	klog.V(2).Infof("All remote host collectors completed")

	defer func() {
		// TODO:
		// delete the config map
		// delete the remote pods
	}()

	// aggregate results
	for node, logs := range nodeLogs {
		var nodeResult map[string]string
		if err := json.Unmarshal(logs, &nodeResult); err != nil {
			// TODO: error handling
			return nil
		}
		for file, data := range nodeResult {
			// trim host-collectors/ prefix
			file = strings.TrimPrefix(file, "host-collectors/")
			err := output.SaveResult(bundlePath, fmt.Sprintf("host-collectors/%s/%s", node, file), bytes.NewBufferString(data))
			if err != nil {
				// TODO: error handling
				return nil
			}
		}
	}

	// save node list to bundle for analyzer to use later
	nodeListBytes, err := json.MarshalIndent(nodeList, "", "  ")
	if err != nil {
		// TODO: error handling
		return nil
	}
	err = output.SaveResult(bundlePath, constants.NODE_LIST_FILE, bytes.NewBuffer(nodeListBytes))
	if err != nil {
		// TODO: error handling
		return nil
	}

	return output
}

func createHostCollectorsSpec(hostCollectors []*troubleshootv1beta2.HostCollect) *troubleshootv1beta2.HostCollector {
	return &troubleshootv1beta2.HostCollector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "HostCollector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "remoteHostCollector",
		},
		Spec: troubleshootv1beta2.HostCollectorSpec{
			Collectors: hostCollectors,
		},
	}
}

func convertHostCollectorSpecToJSON(spec *troubleshootv1beta2.HostCollector) (string, error) {
	jsonData, err := json.Marshal(spec)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal Host Collector spec")
	}
	return string(jsonData), nil
}

func createHostCollectorConfigMap(ctx context.Context, clientset kubernetes.Interface, spec string) (*corev1.ConfigMap, error) {
	// TODO: configurable namespaces?
	ns := "default"

	data := map[string]string{
		"collector.json": spec,
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "remote-host-collector-specs-",
			Namespace:    ns,
		},
		Data: data,
	}

	createdConfigMap, err := clientset.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Remote Host Collector Spec ConfigMap")
	}

	return createdConfigMap, nil
}

func createHostCollectorPod(ctx context.Context, clientset kubernetes.Interface, nodeSelector map[string]string, labels map[string]string) (*corev1.Pod, error) {
	ns := "default"
	imageName := "replicated/troubleshoot:latest"
	imagePullPolicy := corev1.PullAlways

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "remote-host-collector-",
			Namespace:    ns,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			NodeSelector:  nodeSelector,
			HostNetwork:   true,
			HostPID:       true,
			HostIPC:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           imageName,
					ImagePullPolicy: imagePullPolicy,
					Name:            "remote-collector",
					Command:         []string{"/bin/bash", "-c"},
					Args:            []string{"while true; do sleep 30; done;"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-root",
							MountPath: "/host",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host-root",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
		},
	}

	createdPod, err := clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Remote Host Collector Pod")
	}

	return createdPod, nil
}

func waitForPodCompletion(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) error {
	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", pod.Name),
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		podEvent, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		for _, containerStatus := range podEvent.Status.ContainerStatuses {
			if containerStatus.Name == "remote-collector" {
				if containerStatus.State.Terminated != nil {
					if containerStatus.State.Terminated.ExitCode == 0 {
						return nil
					}
					return fmt.Errorf("container %s in pod %s failed with exit code %d", containerStatus.Name, pod.Name, containerStatus.State.Terminated.ExitCode)
				}
			}
		}
	}
	return fmt.Errorf("pod %s did not complete", pod.Name)
}

func waitForPodRunning(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) error {
	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", pod.Name),
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		podEvent, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		for _, containerStatus := range podEvent.Status.ContainerStatuses {
			if containerStatus.Name == "remote-collector" {
				if containerStatus.State.Running != nil {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("pod %s did not complete", pod.Name)
}

func getPodLogs(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) ([]byte, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: pod.Spec.Containers[0].Name,
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer logs.Close()

	return io.ReadAll(logs)
}

func streamPodLogs(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod, node string, opts SupportBundleCreateOpts) {

	// todo: timeout

	send := func(msg string) {
		opts.ProgressChan <- fmt.Sprintf("[%s] %s", node, msg)
	}

	// wait for pod container log-tailer to start
	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", pod.Name),
	})
	if err != nil {
		send(errors.Wrap(err, "failed to start pod watcher").Error())
		return
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		podEvent, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		for _, containerStatus := range podEvent.Status.ContainerStatuses {
			if containerStatus.Name == "log-tailer" {
				if containerStatus.State.Running != nil {
					goto StartLogStream
				}
			}
		}
	}

StartLogStream:
	// stream logs from container named log-tailer in the pod
	podLogOpts := corev1.PodLogOptions{
		Container: "log-tailer",
		Follow:    true,
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	logs, err := req.Stream(ctx)
	if err != nil {
		send(errors.Wrap(err, "failed to get log stream").Error())
		return
	}
	defer logs.Close()
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		send(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		send(errors.Wrap(err, "failed to read log stream").Error())
	}
	send("Log stream ended")
}

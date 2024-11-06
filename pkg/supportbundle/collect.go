package supportbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"

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
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

const (
	selectorLabelKey   = "ds-selector-label"
	selectorLabelValue = "remote-host-collector"
)

func runHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {

	var err error
	var collectResult map[string][]byte

	if opts.RunHostCollectorsInPod {
		collectResult, err = runRemoteHostCollectors(ctx, hostCollectors, bundlePath, opts)
		if err != nil {
			return collectResult, err
		}
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

// getExecOutputs executes `collect -` with collector data passed to stdin and returns stdout, stderr and error
func getExecOutputs(
	ctx context.Context, clientConfig *rest.Config, client *kubernetes.Clientset, pod corev1.Pod, collectorData []byte,
) ([]byte, []byte, error) {
	container := pod.Spec.Containers[0].Name

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"/troubleshoot/collect", "-", "--chroot", "/host", "--format", "raw"},
		Container: container,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewBuffer(collectorData),
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func runRemoteHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, bundlePath string, opts SupportBundleCreateOpts) (map[string][]byte, error) {
	output := collect.NewResult()

	clientset, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, err
	}

	// TODO: rbac check

	nodeList, err := getNodeList(clientset, opts)
	if err != nil {
		return nil, err
	}
	klog.V(2).Infof("Node list to run remote host collectors: %s", nodeList.Nodes)

	// create remote pod for each node
	labels := map[string]string{
		"troubleshoot.sh/remote-collector": "true",
	}

	var mu sync.Mutex
	nodeLogs := make(map[string]map[string][]byte)

	ds, err := createHostCollectorDS(ctx, clientset, labels)
	if err != nil {
		return nil, err
	}

	// wait for at least one pod to be scheduled
	err = waitForDS(ctx, clientset, ds)
	if err != nil {
		return nil, err
	}

	klog.V(2).Infof("Created Remote Host Collector Daemonset %s", ds.Name)
	pods, err := clientset.CoreV1().Pods(ds.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector:  selectorLabelKey + "=" + selectorLabelValue,
		TimeoutSeconds: new(int64),
		Limit:          0,
	})
	if err != nil {
		return nil, err
	}

	var eg errgroup.Group

	for _, collectorSpec := range hostCollectors {
		collector, ok := collect.GetHostCollector(collectorSpec, bundlePath)
		if !ok {
			opts.ProgressChan <- "Host collector not found"
			continue
		}

		// Start a span for tracing
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", "Collect"))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			msg := fmt.Sprintf("[%s] Excluding host collector", collector.Title())
			opts.CollectorProgressCallback(opts.ProgressChan, msg)
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		// Send progress event: starting the collector
		msg := fmt.Sprintf("[%s] Running host collector...", collector.Title())
		opts.CollectorProgressCallback(opts.ProgressChan, msg)

		// convert host collectors into a HostCollector spec
		spec := createHostCollectorsSpec([]*troubleshootv1beta2.HostCollect{collectorSpec})
		specJSON, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		klog.V(2).Infof("HostCollector spec: %s", specJSON)
		for _, pod := range pods.Items {
			eg.Go(func() error {
				if err := waitForPodRunning(ctx, clientset, &pod); err != nil {
					return err
				}

				stdout, _, err := getExecOutputs(ctx, opts.KubernetesRestConfig, clientset, pod, specJSON)
				if err != nil {
					// span.SetStatus(codes.Error, err.Error())
					msg := fmt.Sprintf("[%s] Error: %v", collector.Title(), err)
					opts.CollectorProgressCallback(opts.ProgressChan, msg)
					return errors.Wrap(err, "failed to run remote host collector")
				}

				result := map[string]string{}
				if err := json.Unmarshal(stdout, &result); err != nil {
					return err
				}

				// Send progress event: completed successfully
				msg = fmt.Sprintf("[%s] Completed host collector", collector.Title())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)

				// Aggregate the results
				mu.Lock()
				for file, data := range result {
					if nodeLogs[pod.Spec.NodeName] == nil {
						nodeLogs[pod.Spec.NodeName] = make(map[string][]byte)
					}
					nodeLogs[pod.Spec.NodeName][file] = []byte(data)
				}
				mu.Unlock()
				return nil
			})
		}

		err = eg.Wait()
		if err != nil {
			return nil, err
		}
		span.End()
	}

	klog.V(2).Infof("All remote host collectors completed")

	defer func() {
		// TODO:
		// delete the config map
		// delete the remote pods
		// check if the daemonset still exists
		_, err := clientset.AppsV1().DaemonSets(ds.Namespace).Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to verify remote host collector daemonset %s still exists: %v", ds.Name, err)
			return
		}

		if err := clientset.AppsV1().DaemonSets(ds.Namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("Failed to delete remote host collector daemonset %s: %v", ds.Name, err)
		}
	}()

	for node, logs := range nodeLogs {
		for file, data := range logs {
			// trim host-collectors/ prefix
			file = strings.TrimPrefix(file, "host-collectors/")
			err := output.SaveResult(bundlePath, fmt.Sprintf("host-collectors/%s/%s", node, file), bytes.NewBuffer(data))
			if err != nil {
				// TODO: error handling
				return nil, err
			}
		}
	}

	// save node list to bundle for analyzer to use later
	nodeListBytes, err := json.MarshalIndent(nodeList, "", "  ")
	if err != nil {
		// TODO: error handling
		return nil, err
	}
	err = output.SaveResult(bundlePath, constants.NODE_LIST_FILE, bytes.NewBuffer(nodeListBytes))
	if err != nil {
		// TODO: error handling
		return nil, err
	}

	return output, nil
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

func createHostCollectorDS(ctx context.Context, clientset kubernetes.Interface, labels map[string]string) (*appsv1.DaemonSet, error) {
	ns := "default"
	imageName := "replicated/troubleshoot:latest"
	imagePullPolicy := corev1.PullAlways

	labels[selectorLabelKey] = selectorLabelValue

	podSpec := corev1.PodSpec{
		HostNetwork: true,
		HostPID:     true,
		HostIPC:     true,
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
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName("remote-host-collector" + "-"),
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      selectorLabelKey,
						Operator: "In",
						Values:   []string{selectorLabelValue},
					},
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}

	createdDS, err := clientset.AppsV1().DaemonSets(ns).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Remote Host Collector Pod")
	}

	return createdDS, nil
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
				if containerStatus.State.Running != nil && containerStatus.State.Terminated == nil && containerStatus.Ready {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("pod %s did not complete", pod.Name)
}

func waitForDS(ctx context.Context, clientset kubernetes.Interface, ds *appsv1.DaemonSet) error {
	watcher, err := clientset.AppsV1().DaemonSets(ds.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", ds.Name),
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		dsEvent, ok := event.Object.(*appsv1.DaemonSet)
		if !ok {
			continue
		}
		if dsEvent.Status.NumberReady > 0 && dsEvent.Status.DesiredNumberScheduled == dsEvent.Status.NumberReady {
			return nil
		}
	}
	return fmt.Errorf("pod %s did not complete", ds.Name)
}

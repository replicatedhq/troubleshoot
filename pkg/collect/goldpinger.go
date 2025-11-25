package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

var goldpingerImage = "bloomberg/goldpinger:3.10.1"

// Collect goldpinger results from goldpinger service running in a cluster
// The results are stored in goldpinger/check_all.json since we use
// the /check_all endpoint
type CollectGoldpinger struct {
	Collector    *troubleshootv1beta2.Goldpinger
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectGoldpinger) Title() string {
	return getCollectorName(c)
}

func (c *CollectGoldpinger) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectGoldpinger) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()
	var results []byte
	var err error

	// Check if we have goldpinger running in the cluster, if not, lets deploy it and collect results
	namespace := "default"
	if c.Collector.Namespace != "" {
		namespace = c.Collector.Namespace
	}

	url, resources, err := c.DiscoverOrCreateGoldpinger(namespace)
	if err != nil {
		klog.Errorf("Failed to ensure goldpinger is running: %v", err)
		return nil, errors.Wrap(err, "failed to ensure goldpinger is running")
	}
	defer func() {
		if err := c.cleanupResources(resources); err != nil {
			klog.Errorf("Failed to cleanup resources: %v", err)
		}
	}()

	if util.IsInCluster() {
		klog.V(2).Infof("Collector running in cluster, querying goldpinger endpoint straight away")
		results, err = c.fetchCheckAllOutput(url)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to query goldpinger endpoint in cluster: %v", err)
			klog.V(2).Infof("%s", errMsg)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(errMsg)))
			return output, err
		}
	} else {
		klog.V(2).Infof("Launch pod to query goldpinger endpoint then collect results from pod logs")
		results, err = c.runPodAndCollectGPResults(url, progressChan)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to run pod to collect goldpinger results: %v", err)
			klog.V(2).Infof("%s", errMsg)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(errMsg)))
			return output, err
		}
	}

	err = output.SaveResult(c.BundlePath, constants.GP_CHECK_ALL_RESULTS_PATH, bytes.NewBuffer(results))
	return output, err
}

// cleanupResources collects all created resources for later deletion
// If creation of any resource fails, the already created resources
// will be deleted
type createdResources struct {
	Role         *rbacv1.Role
	RoleBinding  *rbacv1.RoleBinding
	DaemonSet    *appsv1.DaemonSet
	ServiceAccnt *corev1.ServiceAccount
	Service      *corev1.Service
}

func (c *CollectGoldpinger) cleanupResources(resources createdResources) error {
	var errs []error
	if resources.Service != nil {
		if err := c.Client.CoreV1().Services(resources.Service.Namespace).Delete(c.Context, resources.Service.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to delete Service %s", resources.Service.Name))
		}
		klog.V(2).Infof("%s Service deleted", resources.Service.Name)
	}

	if resources.DaemonSet != nil {
		if err := c.Client.AppsV1().DaemonSets(resources.DaemonSet.Namespace).Delete(c.Context, resources.DaemonSet.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to delete DaemonSet %s", resources.DaemonSet.Name))
		}
		klog.V(2).Infof("%s DaemonSet deleted", resources.DaemonSet.Name)
	}

	if resources.ServiceAccnt != nil {
		if err := c.Client.CoreV1().ServiceAccounts(resources.ServiceAccnt.Namespace).Delete(c.Context, resources.ServiceAccnt.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to delete ServiceAccount %s", resources.ServiceAccnt.Name))
		}
		klog.V(2).Infof("%s ServiceAccount deleted", resources.ServiceAccnt.Name)
	}

	if resources.RoleBinding != nil {
		if err := c.Client.RbacV1().RoleBindings(resources.RoleBinding.Namespace).Delete(c.Context, resources.RoleBinding.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to delete RoleBinding %s", resources.RoleBinding.Name))
		}
		klog.V(2).Infof("%s RoleBinding deleted", resources.RoleBinding.Name)
	}

	if resources.Role != nil {
		if err := c.Client.RbacV1().Roles(resources.Role.Namespace).Delete(c.Context, resources.Role.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to delete Role %s", resources.Role.Name))
		}
		klog.V(2).Infof("%s Role deleted", resources.Role.Name)
	}

	if len(errs) > 0 {
		return errors.Errorf("failed to cleanup resources: %v", errs)
	}

	return nil
}

func getUrlFromService(svc *corev1.Service) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/check_all", svc.Name, svc.Namespace, svc.Spec.Ports[0].Port)
}

func parseCollectDelay(delay, defaultDelay string) (time.Duration, error) {
	if delay == "" {
		delay = defaultDelay
	}
	return time.ParseDuration(delay)
}

func (c *CollectGoldpinger) DiscoverOrCreateGoldpinger(ns string) (string, createdResources, error) {
	// Check if goldpinger is running in the cluster by searching for goldpinger's service
	ret := createdResources{}
	gpSvc, err := c.getGoldpingerService(ns)
	if err != nil {
		return "", ret, errors.Wrap(err, "failed to get goldpinger service")
	}

	if gpSvc != nil {
		klog.V(2).Infof("Goldpinger service already exists")
		// By default, no delay needed if goldpinger service already exists
		delay, err := parseCollectDelay(c.Collector.CollectDelay, "0s")
		if err != nil {
			return "", ret, errors.Wrap(err, "failed to parse duration")
		}
		time.Sleep(delay)
		return getUrlFromService(gpSvc), ret, nil
	}

	// If we deploy GP, we need to wait for it to ping pods
	// Defaults to REFRESH_INTERVAL + CHECK_ALL_TIMEOUT
	delay, err := parseCollectDelay(c.Collector.CollectDelay, "6s")
	if err != nil {
		return "", ret, errors.Wrap(err, "failed to parse duration")
	}

	serviceAccountName := c.Collector.ServiceAccountName
	if serviceAccountName == "" {
		serviceAccountName = "ts-goldpinger-serviceaccount"

		svcAcc, err := c.ensureGoldpingerServiceAccount(ns, serviceAccountName)
		if err != nil {
			return "", ret, errors.Wrap(err, "failed to create goldpinger service account")
		}
		ret.ServiceAccnt = svcAcc
		klog.V(2).Infof("%s ServiceAccount created", svcAcc.Name)

		r, err := c.ensureGoldpingerRole(ns)
		if err != nil {
			return "", ret, errors.Wrap(err, "failed to create goldpinger role")
		}
		ret.Role = r
		klog.V(2).Infof("%s Role created", r.Name)

		rb, err := c.ensureGoldpingerRoleBinding(ns)
		if err != nil {
			return "", ret, errors.Wrap(err, "failed to create goldpinger role binding")
		}
		ret.RoleBinding = rb
		klog.V(2).Infof("%s RoleBinding created", rb.Name)
	} else {
		if err := checkForExistingServiceAccount(c.Context, c.Client, ns, serviceAccountName); err != nil {
			return "", ret, err
		}
	}

	ds, err := c.ensureGoldpingerDaemonSet(ns, serviceAccountName)
	if err != nil {
		return "", ret, errors.Wrap(err, "failed to create goldpinger daemonset")
	}
	ret.DaemonSet = ds
	klog.V(2).Infof("%s DaemonSet created", ds.Name)

	// block till DaemonSet has right number of scheduled Pods
	timeoutCtx, cancel := context.WithTimeout(c.Context, defaultTimeout)
	defer cancel()

	err = waitForDaemonSetPods(timeoutCtx, c.Client, ds)
	if err != nil {
		return "", ret, errors.Wrapf(err, "failed to wait for %s DaemonSet pods", ds.Name)
	}
	klog.V(2).Infof("DaemonSet %s has desired number of pods", ds.Name)

	time.Sleep(delay)

	svc, err := c.ensureGoldpingerService(ns)
	if err != nil {
		return "", ret, errors.Wrap(err, "failed to create goldpinger service")
	}
	klog.V(2).Infof("%s Service created", svc.Name)
	ret.Service = svc

	return getUrlFromService(svc), ret, nil
}

func (c *CollectGoldpinger) ensureGoldpingerServiceAccount(ns, name string) (*corev1.ServiceAccount, error) {
	svcAcc := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	created, err := c.Client.CoreV1().ServiceAccounts(ns).Create(c.Context, svcAcc, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	// If the service account already exists, retrieve it
	if kerrors.IsAlreadyExists(err) {
		return c.Client.CoreV1().ServiceAccounts(ns).Get(c.Context, name, metav1.GetOptions{})
	}

	return created, nil
}

func (c *CollectGoldpinger) ensureGoldpingerRole(ns string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ts-goldpinger-role",
			Namespace: ns,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	created, err := c.Client.RbacV1().Roles(ns).Create(c.Context, role, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	// If the role already exists, retrieve it
	if kerrors.IsAlreadyExists(err) {
		return c.Client.RbacV1().Roles(ns).Get(c.Context, role.Name, metav1.GetOptions{})
	}

	return created, nil
}

func (c *CollectGoldpinger) ensureGoldpingerRoleBinding(ns string) (*rbacv1.RoleBinding, error) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ts-goldpinger-rolebinding",
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "ts-goldpinger-serviceaccount",
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "ts-goldpinger-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	created, err := c.Client.RbacV1().RoleBindings(ns).Create(c.Context, roleBinding, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	// If the role binding already exists, retrieve it
	if kerrors.IsAlreadyExists(err) {
		return c.Client.RbacV1().RoleBindings(ns).Get(c.Context, roleBinding.Name, metav1.GetOptions{})
	}

	return created, nil
}

func (c *CollectGoldpinger) ensureGoldpingerDaemonSet(ns, svcAccName string) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}

	if c.Collector.Image != "" {
		goldpingerImage = c.Collector.Image
	}

	ds.ObjectMeta = metav1.ObjectMeta{
		Name:      "ts-goldpinger",
		Namespace: ns,
		Labels:    gpNameLabels(),
	}

	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: gpNameLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:    gpNameLabels(),
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				PriorityClassName:  "system-node-critical",
				ServiceAccountName: svcAccName,
				Containers: []corev1.Container{
					{
						Name:            "goldpinger-daemon",
						Image:           goldpingerImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name: "HOSTNAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
							{
								Name:  "REFRESH_INTERVAL",
								Value: "3", // Refresh interval in seconds. Its not a duration, its a number
							},
							{
								Name:  "CHECK_ALL_TIMEOUT",
								Value: "3s",
							},
							{
								Name:  "HOST",
								Value: "0.0.0.0",
							},
							{
								Name:  "PORT",
								Value: "8080",
							},
							{
								Name:  "LABEL_SELECTOR",
								Value: gpNameLabelSelector(),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							ReadOnlyRootFilesystem: ptr.To(true),
							RunAsNonRoot:           ptr.To(true),
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 8080,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/",
									Port: intstr.FromString("http"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/",
									Port: intstr.FromString("http"),
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{
					FSGroup:      ptr.To(int64(2000)),
					RunAsNonRoot: ptr.To(true),
					RunAsUser:    ptr.To(int64(1000)),
					SeccompProfile: &corev1.SeccompProfile{
						Type: "RuntimeDefault",
					},
				},
			},
		},
	}

	created, err := c.Client.AppsV1().DaemonSets(ns).Create(c.Context, ds, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	// If the daemonset already exists, retrieve it
	if kerrors.IsAlreadyExists(err) {
		return c.Client.AppsV1().DaemonSets(ns).Get(c.Context, ds.Name, metav1.GetOptions{})
	}

	return created, nil
}

func (c *CollectGoldpinger) ensureGoldpingerService(ns string) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ts-goldpinger",
			Namespace: ns,
			Labels:    gpNameLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
			Selector: gpNameLabels(),
		},
	}

	created, err := c.Client.CoreV1().Services(ns).Create(c.Context, svc, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	// If the service already exists, retrieve it
	if kerrors.IsAlreadyExists(err) {
		return c.Client.CoreV1().Services(ns).Get(c.Context, svc.Name, metav1.GetOptions{})
	}

	return created, nil
}

func (c *CollectGoldpinger) getGoldpingerService(ns string) (*corev1.Service, error) {
	svcs, err := c.Client.CoreV1().Services(ns).List(c.Context, metav1.ListOptions{
		LabelSelector: gpNameLabelSelector(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list goldpinger services")
	}

	if len(svcs.Items) == 0 {
		return nil, nil
	}

	return &svcs.Items[0], nil
}

func (c *CollectGoldpinger) fetchCheckAllOutput(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: time.Minute, // Long enough timeout
	}

	req, err := http.NewRequestWithContext(c.Context, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *CollectGoldpinger) runPodAndCollectGPResults(url string, progressChan chan<- interface{}) ([]byte, error) {
	namespace := "default"
	serviceAccountName := ""
	image := constants.GP_DEFAULT_IMAGE

	var imagePullSecret *troubleshootv1beta2.ImagePullSecrets

	if c.Collector.PodLaunchOptions != nil {
		if c.Collector.PodLaunchOptions.Namespace != "" {
			namespace = c.Collector.PodLaunchOptions.Namespace
		}

		if c.Collector.PodLaunchOptions.ServiceAccountName != "" {
			serviceAccountName = c.Collector.PodLaunchOptions.ServiceAccountName
			if err := checkForExistingServiceAccount(c.Context, c.Client, namespace, serviceAccountName); err != nil {
				return nil, err
			}
		}

		if c.Collector.PodLaunchOptions.Image != "" {
			image = c.Collector.PodLaunchOptions.Image
		}
		imagePullSecret = c.Collector.PodLaunchOptions.ImagePullSecret
	}

	runPodCollectorName := "ts-goldpinger-collector"
	collectorContainerName := "collector"
	runPodSpec := &troubleshootv1beta2.RunPod{
		CollectorMeta: troubleshootv1beta2.CollectorMeta{
			CollectorName: runPodCollectorName,
		},
		Name:            runPodCollectorName,
		Namespace:       namespace,
		Timeout:         time.Minute.String(),
		ImagePullSecret: imagePullSecret,
		PodSpec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
			Containers: []corev1.Container{
				{
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Name:            collectorContainerName,
					Command:         []string{"wget"},
					Args:            []string{"-q", "-O-", url},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	rbacErrors := c.GetRBACErrors()
	// Pass an empty bundle path since we don't need to save the results
	runPodCollector := &CollectRunPod{runPodSpec, "", c.Collector.Namespace, c.ClientConfig, c.Client, c.Context, rbacErrors}

	output, err := runPodCollector.Collect(progressChan)
	if err != nil {
		return nil, err
	}

	// Check if the collector container exited with an error
	var pod corev1.Pod
	err = json.Unmarshal(output[fmt.Sprintf("%s/%s.json", runPodCollectorName, runPodCollectorName)], &pod)
	if err != nil {
		return nil, err
	}

	var terminationError *corev1.ContainerStateTerminated
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == collectorContainerName && status.State.Terminated != nil {
			if status.State.Terminated.ExitCode != 0 {
				terminationError = status.State.Terminated
			}
		}
	}

	podLogs := output[fmt.Sprintf("%s/%s.log", runPodCollectorName, runPodCollectorName)]
	if terminationError != nil {
		m := map[string]string{
			"podName":  pod.Name,
			"exitCode": strconv.Itoa(int(terminationError.ExitCode)),
			"reason":   terminationError.Reason,
			"message":  terminationError.Message,
			"logs":     string(podLogs),
		}

		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(b))
	}
	return podLogs, nil
}

func gpNameLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": "goldpinger",
	}
}

func gpNameLabelSelector() string {
	return "app.kubernetes.io/name=goldpinger"
}

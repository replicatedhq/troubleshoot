package collect

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	longhornv1beta1types "github.com/replicatedhq/troubleshoot/pkg/longhorn/apis/longhorn/v1beta1"
	longhornv1beta1 "github.com/replicatedhq/troubleshoot/pkg/longhorn/client/clientset/versioned/typed/longhorn/v1beta1"
	longhorntypes "github.com/replicatedhq/troubleshoot/pkg/longhorn/types"
	"gopkg.in/yaml.v2"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

const (
	DefaultLonghornNamespace = "longhorn-system"
)

var checksumRX = regexp.MustCompile(`(\S+)\s+(\S+)`)

type CollectLonghorn struct {
	Collector    *troubleshootv1beta2.Longhorn
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectLonghorn) Title() string {
	return getCollectorName(c)
}

func (c *CollectLonghorn) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectLonghorn) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectLonghorn) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	ctx := context.TODO()

	ns := DefaultLonghornNamespace
	if c.Collector.Namespace != "" {
		ns = c.Collector.Namespace
	}

	client, err := longhornv1beta1.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create longhorn client")
	}

	output := NewResult()
	var mtx sync.Mutex

	// collect nodes.longhorn.io
	nodes, err := client.Nodes(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apiErr, ok := err.(*apiErrors.StatusError); ok {
			if apiErr.ErrStatus.Code == http.StatusNotFound {
				klog.Error("list nodes.longhorn.io not found")
				return NewResult(), nil
			}
		}
		return nil, errors.Wrap(err, "list nodes.longhorn.io")
	}
	dir := GetLonghornNodesDirectory(ns)
	for _, node := range nodes.Items {
		b, err := yaml.Marshal(node)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal node %s", node.Name)
		}
		key := filepath.Join(dir, node.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect volumes.longhorn.io
	volumes, err := client.Volumes(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list volumes.longhorn.io")
	}
	dir = GetLonghornVolumesDirectory(ns)
	for _, volume := range volumes.Items {
		b, err := yaml.Marshal(volume)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal volume %s", volume.Name)
		}
		key := filepath.Join(dir, volume.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect replicas.longhorn.io
	replicas, err := client.Replicas(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list replicas.longhorn.io")
	}
	dir = GetLonghornReplicasDirectory(ns)
	for _, replica := range replicas.Items {
		b, err := yaml.Marshal(replica)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal replica %s", replica.Name)
		}
		key := filepath.Join(dir, replica.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect engines.longhorn.io
	engines, err := client.Engines(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list engines.longhorn.io")
	}
	dir = GetLonghornEnginesDirectory(ns)
	for _, engine := range engines.Items {
		b, err := yaml.Marshal(engine)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal engine %s", engine.Name)
		}
		key := filepath.Join(dir, engine.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect engineimages.longhorn.io
	engineImages, err := client.EngineImages(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list engineimages.longhorn.io")
	}
	dir = GetLonghornEngineImagesDirectory(ns)
	for _, engineImage := range engineImages.Items {
		b, err := yaml.Marshal(engineImage)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal engineimage %s", engineImage.Name)
		}
		key := filepath.Join(dir, engineImage.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect instancemanagers.longhorn.io
	instanceManagers, err := client.InstanceManagers(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list instancemanagers.longhorn.io")
	}
	dir = GetLonghornInstanceManagersDirectory(ns)
	for _, instanceManager := range instanceManagers.Items {
		b, err := yaml.Marshal(instanceManager)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal instancemanager %s", instanceManager.Name)
		}
		key := filepath.Join(dir, instanceManager.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect backingimagemanagers.longhorn.io
	backingImageManagers, err := client.BackingImageManagers(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list backingimagemanagers.longhorn.io")
	}
	dir = GetLonghornBackingImageManagersDirectory(ns)
	for _, backingImageManager := range backingImageManagers.Items {
		b, err := yaml.Marshal(backingImageManager)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal backingimagemanager %s", backingImageManager.Name)
		}
		key := filepath.Join(dir, backingImageManager.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect backingimages.longhorn.io
	backingImages, err := client.BackingImages(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list backingimages.longhorn.io")
	}
	dir = GetLonghornBackingImagesDirectory(ns)
	for _, backingImage := range backingImages.Items {
		b, err := yaml.Marshal(backingImage)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal backingimage %s", backingImage.Name)
		}
		key := filepath.Join(dir, backingImage.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect sharemanagers.longhorn.io
	shareManagers, err := client.ShareManagers(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list sharemagemanagers.longhorn.io")
	}
	dir = GetLonghornShareManagersDirectory(ns)
	for _, shareManager := range shareManagers.Items {
		b, err := yaml.Marshal(shareManager)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal sharemanager %s", shareManager.Name)
		}
		key := filepath.Join(dir, shareManager.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect settings
	settings, err := client.Settings(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list settings.longhorn.io")
	}
	settingsMap := map[string]string{}
	for _, setting := range settings.Items {
		settingsMap[setting.Name] = setting.Value
	}
	settingsKey := GetLonghornSettingsFile(ns)
	settingsB, err := yaml.Marshal(settingsMap)
	if err != nil {
		return nil, errors.Wrap(err, "marshal longhorn settings")
	}
	output.SaveResult(c.BundlePath, settingsKey, bytes.NewBuffer(settingsB))

	err = c.collectLonghornLogs(ns, output, progressChan)
	if err != nil {
		return nil, errors.Wrap(err, "collect longhorn logs")
	}

	// https://longhorn.io/docs/1.1.1/advanced-resources/data-recovery/corrupted-replica/

	// There is one instance manager replica pod per node. To checksum a replica we will
	// exec into that pod and get the sha256sum of all files in the replica data directory.
	var replicaPodsByNode map[string]string

	var wg sync.WaitGroup

	for _, volume := range volumes.Items {
		if volume.Status.State != longhorntypes.VolumeStateDetached {
			// cannot checksum volumes in use
			continue
		}

		var volReplicas []longhornv1beta1types.Replica
		for _, replica := range replicas.Items {
			if replica.Spec.InstanceSpec.VolumeName != volume.Name {
				continue
			}
			if replica.Spec.InstanceSpec.NodeID == "" {
				continue
			}
			volReplicas = append(volReplicas, replica)
		}
		if len(volReplicas) <= 1 {
			// no reason to checksum volumes with a single replica
			continue
		}

		// At this point we've found a detached volume with multiple replicas so we have to checksum
		// each replica.

		// First initialize the map of nodes to pods we will exec into
		if replicaPodsByNode == nil {
			pods, err := ListInstanceManagerReplicaPods(ctx, c.ClientConfig, ns)
			if err != nil {
				return nil, err
			}
			replicaPodsByNode = pods
		}

		for _, replica := range volReplicas {
			// Find the name of the instance manager replica pod running on the node where this
			// replica is scheduled
			podName := replicaPodsByNode[replica.Spec.InstanceSpec.NodeID]
			if podName == "" {
				continue
			}

			wg.Add(1)
			go func(repl *longhornv1beta1types.Replica, pName string, cfg *rest.Config, vol *longhornv1beta1types.Volume) {
				defer wg.Done()
				checksums, err := GetLonghornReplicaChecksum(cfg, *repl, pName)
				if err != nil {
					klog.Errorf("Failed to get replica %q checksum: %v", repl.Name, err)
					return
				}
				volsDir := GetLonghornVolumesDirectory(ns)
				key := filepath.Join(volsDir, vol.Name, "replicachecksums", repl.Name+".txt")

				// Locking is required because the output object is shared between goroutines
				mtx.Lock()
				output.SaveResult(c.BundlePath, key, bytes.NewBuffer([]byte(checksums)))
				mtx.Unlock()

				// make separate new copies of CR objects so as not to share internal memory
				// of objects between goroutines
			}(replica.DeepCopy(), podName, rest.CopyConfig(c.ClientConfig), volume.DeepCopy())
		}
	}

	wg.Wait()

	return output, nil
}

func (c *CollectLonghorn) collectLonghornLogs(namespace string, results CollectorResult, progressChan chan<- interface{}) error {
	// logs of all pods in namespace
	logsCollectorSpec := &troubleshootv1beta2.Logs{
		Selector:  []string{""},
		Name:      GetLonghornLogsDirectory(namespace), // Logs (symlinks) will be stored in this directory
		Namespace: namespace,
	}

	rbacErrors := c.GetRBACErrors()
	logsCollector := &CollectLogs{logsCollectorSpec, c.BundlePath, namespace, c.ClientConfig, c.Client, c.Context, nil, rbacErrors}

	logs, err := logsCollector.Collect(progressChan)
	if err != nil {
		return err
	}

	// Add logs collector results to the rest of
	// the longhorn collector results for later consumption
	results.AddResult(logs)

	return nil
}

func GetLonghornNodesDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/nodes", namespace)
}

func GetLonghornVolumesDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/volumes", namespace)
}

func GetLonghornReplicasDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/replicas", namespace)
}

func GetLonghornEnginesDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/engines", namespace)
}

func GetLonghornEngineImagesDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/engineimages", namespace)
}

func GetLonghornInstanceManagersDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/instancemanagers", namespace)
}

func GetLonghornBackingImageManagersDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/backingimagemanagers", namespace)
}

func GetLonghornBackingImagesDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/backingimages", namespace)
}

func GetLonghornShareManagersDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/sharemanagers", namespace)
}

func GetLonghornSettingsFile(namespace string) string {
	return fmt.Sprintf("longhorn/%s/settings.yaml", namespace)
}

func GetLonghornLogsDirectory(namespace string) string {
	return fmt.Sprintf("longhorn/%s/logs", namespace)
}

func GetLonghornReplicaChecksum(clientConfig *rest.Config, replica longhornv1beta1types.Replica, podName string) (string, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return "", err
	}
	dir := fmt.Sprintf("/host/var/lib/longhorn/replicas/%s", replica.Spec.DataDirectoryName)

	req := client.
		CoreV1().
		RESTClient().
		Post().
		Namespace(replica.Namespace).
		Name(podName).
		Resource("pods").
		SubResource("exec").
		Param("container", "replica-manager").
		Param("stdout", "true").
		Param("stdin", "true").
		Param("command", "/bin/bash").
		Param("command", "-c").
		Param("command", fmt.Sprintf("if [ -d %s ]; then md5sum %s/*; fi", dir, dir))

	executor, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return "", errors.Wrapf(err, "create remote exec")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", errors.Wrapf(err, "stream remote exec: %s", stderr.String())
	}

	return stdout.String(), nil
}

// Returns a map of nodeName:podName
func ListInstanceManagerReplicaPods(ctx context.Context, clientConfig *rest.Config, namespace string) (map[string]string, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{
		LabelSelector: "longhorn.io/instance-manager-type=replica",
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, options)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}
	for _, pod := range pods.Items {
		node := pod.Labels["longhorn.io/node"]
		out[node] = pod.Name
	}

	return out, nil
}

func ParseReplicaChecksum(data []byte) (map[string]string, error) {
	buf := bytes.NewBuffer(data)
	scanner := bufio.NewScanner(buf)

	out := map[string]string{}

	for scanner.Scan() {
		matches := checksumRX.FindStringSubmatch(scanner.Text())
		if len(matches) < 3 {
			continue
		}
		filename := filepath.Base(matches[2])
		out[filename] = matches[1]
	}

	return out, scanner.Err()
}

package collect

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	longhornv1beta1types "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	longhornv1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/client/clientset/versioned/typed/longhorn/v1beta1"
	longhorntypes "github.com/longhorn/longhorn-manager/types"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	DefaultLonghornNamespace = "longhorn-system"
)

var checksumRX = regexp.MustCompile(`(\S+)\s+(\S+)`)

func Longhorn(c *Collector, longhornCollector *troubleshootv1beta2.Longhorn) (map[string][]byte, error) {
	ctx := context.TODO()

	ns := DefaultLonghornNamespace
	if longhornCollector.Namespace != "" {
		ns = longhornCollector.Namespace
	}

	client, err := longhornv1beta1.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create longhorn client")
	}

	final := map[string][]byte{}

	// collect nodes.longhorn.io
	nodes, err := client.Nodes(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list nodes.longhorn.io")
	}
	dir := GetLonghornNodesDirectory(ns)
	for _, node := range nodes.Items {
		b, err := yaml.Marshal(node)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal node %s", node.Name)
		}
		key := filepath.Join(dir, node.Name+".yaml")
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
		final[key] = b
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
	final[settingsKey] = settingsB

	// logs of all pods in namespace
	logsCollector := &troubleshootv1beta2.Logs{
		Selector:  []string{""},
		Namespace: ns,
	}
	logs, err := Logs(c, logsCollector)
	if err != nil {
		return nil, errors.Wrap(err, "collect longhorn logs")
	}
	logsDir := GetLonghornLogsDirectory(ns)
	for key, log := range logs {
		key = filepath.Join(logsDir, key)
		final[key] = log
	}

	// https://longhorn.io/docs/1.1.1/advanced-resources/data-recovery/corrupted-replica/

	// There is one instance manager replica pod per node. To checksum a replica we will
	// exec into that pod and get the sha256sum of all files in the replica data directory.
	var replicaPodsByNode map[string]string

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

			checksums, err := GetLonghornReplicaChecksum(c.ClientConfig, replica, podName)
			if err != nil {
				return nil, err
			}
			volsDir := GetLonghornVolumesDirectory(ns)
			key := filepath.Join(volsDir, volume.Name, "replicachecksums", replica.Name+".txt")
			final[key] = []byte(checksums)
		}
	}

	return final, nil
}

func GetLonghornNodesDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/nodes", namespace)
	}
	return "longhorn/nodes"
}

func GetLonghornVolumesDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/volumes", namespace)
	}
	return "longhorn/volumes"
}

func GetLonghornReplicasDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/replicas", namespace)
	}
	return "longhorn/replicas"
}

func GetLonghornEnginesDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/engines", namespace)
	}
	return "longhorn/engines"
}

func GetLonghornEngineImagesDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/engineimages", namespace)
	}
	return "longhorn/engineimages"
}

func GetLonghornInstanceManagersDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/instancemanagers", namespace)
	}
	return "longhorn/instancemanagers"
}

func GetLonghornBackingImageManagersDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/backingimagemanagers", namespace)
	}
	return "longhorn/backingimagemanagers"
}

func GetLonghornBackingImagesDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/backingimages", namespace)
	}
	return "longhorn/backingimages"
}

func GetLonghornShareManagersDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/sharemanagers", namespace)
	}
	return "longhorn/sharemanagers"
}

func GetLonghornSettingsFile(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/settings.yaml", namespace)
	}
	return "longhorn/settings.yaml"
}

func GetLonghornLogsDirectory(namespace string) string {
	if namespace != DefaultLonghornNamespace {
		return fmt.Sprintf("longhorn/%s/logs", namespace)
	}
	return "longhorn/logs"
}

func GetLonghornReplicaChecksum(clientConfig *rest.Config, replica longhornv1beta1types.Replica, podName string) (string, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return "", err
	}

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
		Param("command", fmt.Sprintf("sha256sum /host/var/lib/longhorn/replicas/%s/*", replica.Spec.DataDirectoryName))

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

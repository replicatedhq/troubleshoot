package collect

import (
	"context"
	"fmt"
	"path/filepath"

	longhornv1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/client/clientset/versioned/typed/longhorn/v1beta1"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultLonghornNamespace = "longhorn-system"
)

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

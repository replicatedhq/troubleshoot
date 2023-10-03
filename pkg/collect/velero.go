package collect

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	veleroclient "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	DefaultVeleroNamespace = "velero"
)

type CollectVelero struct {
	Collector    *troubleshootv1beta2.Velero
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

type VeleroCommand struct {
	ID             string
	Command        []string
	Args           []string
	Format         string
	DefaultTimeout string
}

var VeleroCommands = []VeleroCommand{
	{
		ID:             "get-backups",
		Command:        []string{"/velero", "get", "backups"},
		Args:           []string{"-o", "yaml"},
		Format:         "yaml",
		DefaultTimeout: "30s",
	},
	{
		ID:             "get-restores",
		Command:        []string{"/velero", "get", "restores"},
		Args:           []string{"-o", "yaml"},
		Format:         "yaml",
		DefaultTimeout: "30s",
	},
	{
		ID:             "describe-backups",
		Command:        []string{"/velero", "describe", "backups"},
		Args:           []string{"--details", "--colorized", "no"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
	{
		ID:             "describe-restores",
		Command:        []string{"/velero", "describe", "restores"},
		Args:           []string{"--details", "--colorized", "no"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
}

func (c *CollectVelero) Title() string {
	return getCollectorName(c)
}

func (c *CollectVelero) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectVelero) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	ctx := context.TODO()

	ns := DefaultVeleroNamespace
	if c.Collector.Namespace != "" {
		ns = c.Collector.Namespace
	}

	pod, err := findVeleroPod(ctx, c, c.Collector.Namespace)
	if err != nil {
		return nil, err
	}
	output := NewResult()

	// collect output from velero binary
	if pod != nil {
		for _, command := range VeleroCommands {
			err := veleroCommandExec(ctx, progressChan, c, c.Collector, pod, command, output)
			if err != nil {
				pathPrefix := GetVeleroCollectorFilepath(c.Collector.CollectorName, c.Collector.Namespace)
				dstFileName := path.Join(pathPrefix, fmt.Sprintf("%s.%s-error", command.ID, command.Format))
				output.SaveResult(c.BundlePath, dstFileName, strings.NewReader(err.Error()))
			}
		}
	}

	veleroclient, err := veleroclient.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create velero client")
	}

	// collect backupstoragelocations.velero.io
	backupStorageLocations, err := veleroclient.VeleroV1().BackupStorageLocations(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apiErr, ok := err.(*apiErrors.StatusError); ok {
			if apiErr.ErrStatus.Code == http.StatusNotFound {
				klog.V(2).Infof("failed to list backup storage locations in namespace %s", c.Collector.Namespace)
				return NewResult(), nil
			}
		}
		return nil, errors.Wrap(err, "list backupstoragelocations.velero.io")
	}
	dir := GetVeleroBackupStorageLocationsDirectory(ns)
	for _, backupStorageLocation := range backupStorageLocations.Items {
		b, err := yaml.Marshal(backupStorageLocation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal backup storage location %s", backupStorageLocation.Name)
		}
		key := filepath.Join(dir, backupStorageLocation.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect backups.velero.io
	backups, err := veleroclient.VeleroV1().Backups(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apiErr, ok := err.(*apiErrors.StatusError); ok {
			if apiErr.ErrStatus.Code == http.StatusNotFound {
				klog.V(2).Infof("failed to list backups in namespace %s", c.Collector.Namespace)
				return NewResult(), nil
			}
		}
		return nil, errors.Wrap(err, "list backups.velero.io")
	}
	dir = GetVeleroBackupsDirectory(ns)
	for _, backup := range backups.Items {
		b, err := yaml.Marshal(backup)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal backup %s", backup.Name)
		}
		key := filepath.Join(dir, backup.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect backuprepositories.velero.io
	backupRepositories, err := veleroclient.VeleroV1().BackupRepositories(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apiErr, ok := err.(*apiErrors.StatusError); ok {
			if apiErr.ErrStatus.Code == http.StatusNotFound {
				klog.V(2).Infof("failed to list backuprepositories in namespace %s", c.Collector.Namespace)
				return NewResult(), nil
			}
		}
		return nil, errors.Wrap(err, "list backuprepositories.velero.io")
	}
	dir = GetVeleroBackupRepositoriesDirectory(ns)
	for _, backupRepository := range backupRepositories.Items {
		b, err := yaml.Marshal(backupRepository)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal backup repository %s", backupRepository.Name)
		}
		key := filepath.Join(dir, backupRepository.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect restores.velero.io
	restores, err := veleroclient.VeleroV1().Restores(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list restores.velero.io")
	}
	dir = GetVeleroRestoresDirectory(ns)
	for _, restore := range restores.Items {
		b, err := yaml.Marshal(restore)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal restore %s", restore.Name)
		}
		key := filepath.Join(dir, restore.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect resticrepositories.velero.io
	// resticRepositories, err := veleroclient.VeleroV1().ResticRepositories(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return nil, errors.Wrap(err, "list resticrepositories.velero.io")
	// }
	// dir = GetVeleroResticRepositoriesDirectory(ns)
	// for _, resticRepository := range resticRepositories.Items {
	// 	b, err := yaml.Marshal(resticRepository)
	// 	if err != nil {
	// 		return nil, errors.Wrapf(err, "failed to marshal restic repository %s", resticRepository.Name)
	// 	}
	// 	key := filepath.Join(dir, resticRepository.Name+".yaml")
	// 	output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	// }

	// collect deletebackuprequests.velero.io
	deleteBackupRequests, err := veleroclient.VeleroV1().DeleteBackupRequests(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list deletebackuprequests.velero.io")
	}
	dir = GetVeleroDeleteBackupRequestsDirectory(ns)
	for _, deleteBackupRequest := range deleteBackupRequests.Items {
		b, err := yaml.Marshal(deleteBackupRequest)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal delete backup request %s", deleteBackupRequest.Name)
		}
		key := filepath.Join(dir, deleteBackupRequest.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect downloadrequests.velero.io
	downloadRequests, err := veleroclient.VeleroV1().DownloadRequests(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list downloadrequests.velero.io")
	}
	dir = GetVeleroDownloadRequestsDirectory(ns)
	for _, downloadRequest := range downloadRequests.Items {
		b, err := yaml.Marshal(downloadRequest)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal download request %s", downloadRequest.Name)
		}
		key := filepath.Join(dir, downloadRequest.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect podvolumebackups.velero.io
	podVolumeBackups, err := veleroclient.VeleroV1().PodVolumeBackups(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list podvolumebackups.velero.io")
	}
	dir = GetVeleroPodVolumeBackupsDirectory(ns)
	for _, podVolumeBackup := range podVolumeBackups.Items {
		b, err := yaml.Marshal(podVolumeBackup)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal pod volume backup %s", podVolumeBackup.Name)
		}
		key := filepath.Join(dir, podVolumeBackup.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect podvolumerestores.velero.io
	podVolumeRestores, err := veleroclient.VeleroV1().PodVolumeRestores(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list podvolumerestores.velero.io")
	}
	dir = GetVeleroPodVolumeRestoresDirectory(ns)
	for _, podVolumeRestore := range podVolumeRestores.Items {
		b, err := yaml.Marshal(podVolumeRestore)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal pod volume restore %s", podVolumeRestore.Name)
		}
		key := filepath.Join(dir, podVolumeRestore.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect restores.velero.io
	restores, err = veleroclient.VeleroV1().Restores(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list restores.velero.io")
	}
	dir = GetVeleroRestoresDirectory(ns)
	for _, restore := range restores.Items {
		b, err := yaml.Marshal(restore)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal restore %s", restore.Name)
		}
		key := filepath.Join(dir, restore.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect schedules.velero.io
	schedules, err := veleroclient.VeleroV1().Schedules(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list schedules.velero.io")
	}
	dir = GetVeleroSchedulesDirectory(ns)
	for _, schedule := range schedules.Items {
		b, err := yaml.Marshal(schedule)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal schedule %s", schedule.Name)
		}
		key := filepath.Join(dir, schedule.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect serverstatusrequests.velero.io
	serverStatusRequests, err := veleroclient.VeleroV1().ServerStatusRequests(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list serverstatusrequests.velero.io")
	}
	dir = GetVeleroServerStatusRequestsDirectory(ns)
	for _, serverStatusRequest := range serverStatusRequests.Items {
		b, err := yaml.Marshal(serverStatusRequest)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal server status request %s", serverStatusRequest.Name)
		}
		key := filepath.Join(dir, serverStatusRequest.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect volumesnapshotlocations.velero.io
	volumeSnapshotLocations, err := veleroclient.VeleroV1().VolumeSnapshotLocations(c.Collector.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list volumesnapshotlocations.velero.io")
	}
	dir = GetVeleroVolumeSnapshotLocationsDirectory(ns)
	for _, volumeSnapshotLocation := range volumeSnapshotLocations.Items {
		b, err := yaml.Marshal(volumeSnapshotLocation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal volume snapshot location %s", volumeSnapshotLocation.Name)
		}
		key := filepath.Join(dir, volumeSnapshotLocation.Name+".yaml")
		output.SaveResult(c.BundlePath, key, bytes.NewBuffer(b))
	}

	// collect logs
	err = c.collectVeleroLogs(ns, output, progressChan)
	if err != nil {
		return nil, errors.Wrap(err, "collect velero logs")
	}

	return output, nil
}

func veleroCommandExec(ctx context.Context, progressChan chan<- interface{}, c *CollectVelero, veleroCollector *troubleshootv1beta2.Velero, pod *corev1.Pod, command VeleroCommand, output CollectorResult) error {
	timeout := veleroCollector.Timeout
	if timeout == "" {
		timeout = command.DefaultTimeout
	}

	execSpec := &troubleshootv1beta2.Exec{
		Selector:  labelsToSelector(pod.Labels),
		Namespace: pod.Namespace,
		Command:   command.Command,
		Args:      command.Args,
		Timeout:   timeout,
	}

	rbacErrors := c.GetRBACErrors()
	execCollector := &CollectExec{execSpec, c.BundlePath, c.Namespace, c.ClientConfig, c.Client, c.Context, rbacErrors}

	results, err := execCollector.Collect(progressChan)
	if err != nil {
		return errors.Wrap(err, "failed to exec velero command")
	}

	pathPrefix := GetVeleroCollectorFilepath(veleroCollector.CollectorName, veleroCollector.Namespace)
	for srcFilename := range results {
		var dstFileName string
		switch {
		case strings.HasSuffix(srcFilename, "-stdout.txt"):
			dstFileName = path.Join(pathPrefix, fmt.Sprintf("%s.%s", command.ID, command.Format))
		case strings.HasSuffix(srcFilename, "-stderr.txt"):
			dstFileName = path.Join(pathPrefix, fmt.Sprintf("%s-stderr.txt", command.ID))
		case strings.HasSuffix(srcFilename, "-errors.json"):
			dstFileName = path.Join(pathPrefix, fmt.Sprintf("%s-errors.json", command.ID))
		default:
			continue
		}

		err := copyResult(results, output, c.BundlePath, srcFilename, dstFileName)
		if err != nil {
			return errors.Wrap(err, "failed to copy file")
		}
	}

	return nil
}

func findVeleroPod(ctx context.Context, c *CollectVelero, namespace string) (*corev1.Pod, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	pods, _ := listPodsInSelectors(ctx, client, namespace, []string{"deploy=velero"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	klog.Info("velero pod not found in namespace %s", namespace)

	return nil, nil
}

func GetVeleroCollectorFilepath(name, namespace string) string {
	parts := []string{}
	if name != "" {
		parts = append(parts, name)
	}
	if namespace != "" && namespace != DefaultVeleroNamespace {
		parts = append(parts, namespace)
	}
	parts = append(parts, "velero")
	return path.Join(parts...)
}

func (c *CollectVelero) collectVeleroLogs(namespace string, results CollectorResult, progressChan chan<- interface{}) error {
	veleroLogsCollectorSpec := &troubleshootv1beta2.Logs{
		Selector:  []string{""},
		Name:      GetVeleroLogsDirectory(namespace),
		Namespace: namespace,
	}
	rbacErrors := c.GetRBACErrors()
	logsCollector := &CollectLogs{veleroLogsCollectorSpec, c.BundlePath, namespace, c.ClientConfig, c.Client, c.Context, nil, rbacErrors}
	logs, err := logsCollector.Collect(progressChan)
	if err != nil {
		return err
	}
	results.AddResult(logs)
	return nil
}

func GetVeleroBackupsDirectory(namespace string) string {
	return fmt.Sprintf("%s/backups", namespace)
}

func GetVeleroBackupRepositoriesDirectory(namespace string) string {
	return fmt.Sprintf("%s/backuprepositories", namespace)
}

func GetVeleroBackupStorageLocationsDirectory(namespace string) string {
	return fmt.Sprintf("%s/backupstoragelocations", namespace)
}

func GetVeleroDeleteBackupRequestsDirectory(namespace string) string {
	return fmt.Sprintf("%s/deletebackuprequests", namespace)
}

func GetVeleroDownloadRequestsDirectory(namespace string) string {
	return fmt.Sprintf("%s/downloadrequests", namespace)
}

func GetVeleroLogsDirectory(namespace string) string {
	return fmt.Sprintf("%s/logs", namespace)
}

func GetVeleroPodVolumeBackupsDirectory(namespace string) string {
	return fmt.Sprintf("%s/podvolumebackups", namespace)
}

func GetVeleroPodVolumeRestoresDirectory(namespace string) string {
	return fmt.Sprintf("%s/podvolumerestores", namespace)
}

func GetVeleroRestoresDirectory(namespace string) string {
	return fmt.Sprintf("%s/restores", namespace)
}

func GetVeleroSchedulesDirectory(namespace string) string {
	return fmt.Sprintf("%s/schedules", namespace)
}

func GetVeleroServerStatusRequestsDirectory(namespace string) string {
	return fmt.Sprintf("%s/serverstatusrequests", namespace)
}

func GetVeleroVolumeSnapshotLocationsDirectory(namespace string) string {
	return fmt.Sprintf("%s/volumesnapshotlocations", namespace)
}

func GetVeleroResticRepositoriesDirectory(namespace string) string {
	return fmt.Sprintf("%s/resticrepositories", namespace)
}

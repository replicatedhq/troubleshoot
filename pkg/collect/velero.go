package collect

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
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
		Args:           []string{"-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "get-restores",
		Command:        []string{"/velero", "get", "restores"},
		Args:           []string{"-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "describe-backups",
		Command:        []string{"/velero", "describe", "backups"},
		Args:           []string{"--details", "-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "describe-restores",
		Command:        []string{"/velero", "describe", "restores"},
		Args:           []string{"--details", "-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
}

func (c *CollectVelero) Title() string {
	return getCollectorName(c)
}

func (c *CollectVelero) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

// type VeleroOutput struct {
// 	Namespace              string                `json:"namespace"`
// 	Name                   string                `json:"name"`
// 	BackupStorageLocation  string                `json:"backupStorageLocation"`
// 	VolumeSnapshotLocation string                `json:"volumeSnapshotLocation"`
// 	RestoreLocation        VeleroRestoreLocation `json:"restoreLocation"`
// 	ResticRepositories     VeleroResticRepository
// }

func (c *CollectVelero) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	// implement collector
	ctx := context.TODO()

	if c.Collector.Namespace == "" {
		c.Collector.Namespace = DefaultVeleroNamespace
	}

	pod, err := findVeleroPod(ctx, c, c.Collector.Namespace)
	if err != nil {
		return nil, err
	}
	output := NewResult()

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

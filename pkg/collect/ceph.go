package collect

import (
	"context"
	"fmt"
	"os"
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
	DefaultCephNamespace = "rook-ceph"
)

type CephCommand struct {
	ID             string
	Command        []string
	Args           []string
	Format         string
	DefaultTimeout string
}

var CephCommands = []CephCommand{
	{
		ID:             "status",
		Command:        []string{"ceph", "status"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "status-txt",
		Command:        []string{"ceph", "status"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
	{
		ID:             "fs",
		Command:        []string{"ceph", "fs", "status"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "fs-txt",
		Command:        []string{"ceph", "fs", "status"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
	{
		ID:             "fs-ls",
		Command:        []string{"ceph", "fs", "ls"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "osd-status",
		Command:        []string{"ceph", "osd", "status"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "osd-tree",
		Command:        []string{"ceph", "osd", "tree"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "osd-pool",
		Command:        []string{"ceph", "osd", "pool", "ls", "detail"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "health",
		Command:        []string{"ceph", "health", "detail"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "auth",
		Command:        []string{"ceph", "auth", "ls"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "rgw-stats", // the disk usage (and other stats) of each object store bucket
		Command:        []string{"radosgw-admin", "bucket", "stats"},
		Args:           []string{"--rgw-cache-enabled=false"},
		Format:         "json",
		DefaultTimeout: "30s", // include a default timeout because this command will hang if the RGW daemon isn't running/is unhealthy
	},
	{
		ID:      "rbd-du-txt", // the disk usage of each PVC
		Command: []string{"rbd", "du"},
		Args:    []string{"--pool=replicapool"},
		Format:  "txt",
		// On ceph clusters with a lot of data, this command can take a long time to run
		// especially if fast-diff rbd_default_features is disabled
		DefaultTimeout: "60s",
	},
	{
		ID:             "df", // the disk usage of each pool
		Command:        []string{"ceph", "df"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "df-txt", // the disk usage of each pool
		Command:        []string{"ceph", "df"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
	{
		ID:             "osd-df",
		Command:        []string{"ceph", "osd", "df"},
		Args:           []string{"-f", "json-pretty"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "osd-df-txt",
		Command:        []string{"ceph", "osd", "df"},
		Format:         "txt",
		DefaultTimeout: "30s",
	},
}

type CollectCeph struct {
	Collector    *troubleshootv1beta2.Ceph
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectCeph) Title() string {
	return getCollectorName(c)
}

func (c *CollectCeph) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCeph) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectCeph) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	ctx := context.TODO()

	if c.Collector.Namespace == "" {
		c.Collector.Namespace = DefaultCephNamespace
	}

	pod, err := findRookCephToolsPod(ctx, c, c.Collector.Namespace)
	if err != nil {
		return nil, err
	}

	output := NewResult()
	if pod != nil {
		for _, command := range CephCommands {
			err := cephCommandExec(ctx, progressChan, c, c.Collector, pod, command, output)
			if err != nil {
				pathPrefix := GetCephCollectorFilepath(c.Collector.CollectorName, c.Collector.Namespace)
				dstFileName := path.Join(pathPrefix, fmt.Sprintf("%s.%s-error", command.ID, command.Format))
				output.SaveResult(c.BundlePath, dstFileName, strings.NewReader(err.Error()))
			}
		}
	}
	return output, nil
}

func cephCommandExec(ctx context.Context, progressChan chan<- interface{}, c *CollectCeph, cephCollector *troubleshootv1beta2.Ceph, pod *corev1.Pod, command CephCommand, output CollectorResult) error {
	timeout := cephCollector.Timeout
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
		return errors.Wrap(err, "failed to exec command")
	}

	pathPrefix := GetCephCollectorFilepath(cephCollector.CollectorName, cephCollector.Namespace)
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

func findRookCephToolsPod(ctx context.Context, c *CollectCeph, namespace string) (*corev1.Pod, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	pods, _ := listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-tools"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	pods, _ = listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-operator"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	klog.Info("rook ceph tools pod not found")

	return nil, nil
}

func GetCephCollectorFilepath(name, namespace string) string {
	parts := []string{}
	if name != "" {
		parts = append(parts, name)
	}
	if namespace != "" && namespace != DefaultCephNamespace {
		parts = append(parts, namespace)
	}
	parts = append(parts, "ceph")
	return path.Join(parts...)
}

func labelsToSelector(labels map[string]string) []string {
	selector := []string{}
	for key, value := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", key, value))
	}
	return selector
}

func copyResult(srcResult CollectorResult, dstResult CollectorResult, bundlePath string, srcKey string, dstKey string) error {
	reader, err := srcResult.GetReader(bundlePath, srcKey)
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return nil
		}
		return errors.Wrap(err, "failed to get reader")
	}
	defer reader.Close()

	err = dstResult.SaveResult(bundlePath, dstKey, reader)
	if err != nil {
		return errors.Wrap(err, "failed to save file")
	}

	return nil
}

package collect

import (
	"context"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	output := NewResult()

	return output, nil
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
		Command:        []string{"velero", "get", "backups"},
		Args:           []string{"-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "get-restores",
		Command:        []string{"velero", "get", "restores"},
		Args:           []string{"-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
	{
		ID:             "describe-backups",
		Command:        []string{"velero", "describe", "backups"},
		Args:           []string{"--details", "-o", "json"},
		Format:         "json",
		DefaultTimeout: "30s",
	},
}

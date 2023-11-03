package analyzer

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	restic_types "github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnalyzeVelero_BackupRepositories(t *testing.T) {
	type args struct {
		backupRepositories []*velerov1.BackupRepository
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no backup repositories",
			args: args{
				backupRepositories: []*velerov1.BackupRepository{},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Backup Repository configured",
					Message: "No backup repositories configured",
					IsFail:  true,
				},
			},
		},
		{
			name: "1 backup repository and 1 Ready",
			args: args{
				backupRepositories: []*velerov1.BackupRepository{
					{
						Spec: velerov1.BackupRepositorySpec{
							BackupStorageLocation: "default",
							VolumeNamespace:       "velero",
						},
						Status: velerov1.BackupRepositoryStatus{
							Phase: velerov1.BackupRepositoryPhaseReady,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Backup Repository configured",
					Message: "Found 1 backup repositories configured and 1 Ready",
					IsPass:  true,
				},
			},
		},
		{
			name: "2 backup repositories and 1 Ready",
			args: args{
				backupRepositories: []*velerov1.BackupRepository{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default-default-restic-245sd",
							Namespace: "velero",
						},
						Spec: velerov1.BackupRepositorySpec{
							BackupStorageLocation: "default",
							VolumeNamespace:       "velero",
						},
						Status: velerov1.BackupRepositoryStatus{
							Phase: velerov1.BackupRepositoryPhaseReady,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "builders-default-restic-jdtd8",
							Namespace: "velero",
						},
						Spec: velerov1.BackupRepositorySpec{
							BackupStorageLocation: "builders-default-restic-jdtd8",
							VolumeNamespace:       "velero",
						},
						Status: velerov1.BackupRepositoryStatus{
							Phase: velerov1.BackupRepositoryPhaseNotReady,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Backup Repository builders-default-restic-jdtd8",
					Message: "Backup Repository [builders-default-restic-jdtd8] is in phase NotReady",
					IsWarn:  true,
				},
				{
					Title:   "At least 1 Backup Repository configured",
					Message: "Found 2 backup repositories configured and 1 Ready",
					IsPass:  true,
				},
			},
		},
		{
			name: "1 backup repository and none Ready",
			args: args{
				backupRepositories: []*velerov1.BackupRepository{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: "velero",
						},
						Spec: velerov1.BackupRepositorySpec{
							BackupStorageLocation: "default",
							VolumeNamespace:       "velero",
						},
						Status: velerov1.BackupRepositoryStatus{
							Phase: velerov1.BackupRepositoryPhaseNotReady,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Backup Repository default",
					Message: "Backup Repository [default] is in phase NotReady",
					IsWarn:  true,
				},
				{
					Title:   "At least 1 Backup Repository configured",
					Message: "Found 1 configured backup repositories, but none are ready",
					IsWarn:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeBackupRepositories(tt.args.backupRepositories); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeBackupRepositories() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_ResticRepositories(t *testing.T) {
	type args struct {
		resticRepositories []*restic_types.ResticRepository
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no restic repositories",
			args: args{
				resticRepositories: []*restic_types.ResticRepository{},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Restic Repository configured",
					Message: "No restic repositories configured",
					IsFail:  true,
				},
			},
		},
		{
			name: "1 restic repository and 1 Ready",
			args: args{
				resticRepositories: []*restic_types.ResticRepository{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default-default-restic-245sd",
							Namespace: "velero",
						},
						Spec: restic_types.ResticRepositorySpec{
							BackupStorageLocation: "default",
							VolumeNamespace:       "velero",
						},
						Status: restic_types.ResticRepositoryStatus{
							Phase: restic_types.ResticRepositoryPhaseReady,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Restic Repository configured",
					Message: "Found 1 restic repositories configured and 1 Ready",
					IsPass:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeResticRepositories(tt.args.resticRepositories); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeResticRepositories() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_Backups(t *testing.T) {
	type args struct {
		backups []*velerov1.Backup
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no backups",
			args: args{
				backups: []*velerov1.Backup{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "1 backup and 1 Completed",
			args: args{
				backups: []*velerov1.Backup{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "observability-backup",
							Namespace: "velero",
						},
						Spec: velerov1.BackupSpec{
							IncludedNamespaces: []string{"monitoring"},
						},
						Status: velerov1.BackupStatus{
							Phase: velerov1.BackupPhaseCompleted,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero Backups",
					Message: "Found 1 backups",
					IsPass:  true,
				},
			},
		},
		{
			name: "1 backup and 1 Failed",
			args: args{
				backups: []*velerov1.Backup{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "application-backup",
							Namespace: "velero",
						},
						Spec: velerov1.BackupSpec{
							IncludedNamespaces: []string{"shazam"},
						},
						Status: velerov1.BackupStatus{
							Phase: velerov1.BackupPhaseFailed,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Backup application-backup",
					Message: "Backup application-backup phase is Failed",
					IsFail:  true,
				},
				{
					Title:   "Velero Backups",
					Message: "Found 1 backups",
					IsPass:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeBackups(tt.args.backups); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeBackups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_BackupStorageLocations(t *testing.T) {
	type args struct {
		backupStorageLocations []*velerov1.BackupStorageLocation
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no backup storage locations",
			args: args{
				backupStorageLocations: []*velerov1.BackupStorageLocation{},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Backup Storage Location configured",
					Message: "No backup storage locations configured",
					IsFail:  true,
				},
			},
		},
		{
			name: "1 backup storage location and 1 Available",
			args: args{
				backupStorageLocations: []*velerov1.BackupStorageLocation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: "velero",
						},
						Spec: velerov1.BackupStorageLocationSpec{
							Provider: "aws",
						},
						Status: velerov1.BackupStorageLocationStatus{
							Phase: velerov1.BackupStorageLocationPhaseAvailable,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "At least 1 Backup Storage Location configured",
					Message: "Found 1 backup storage locations configured and 1 Available",
					IsPass:  true,
				},
			},
		},
		{
			name: "1 backup storage location and 1 Unavailable",
			args: args{
				backupStorageLocations: []*velerov1.BackupStorageLocation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: "velero",
						},
						Spec: velerov1.BackupStorageLocationSpec{
							Provider: "aws",
						},
						Status: velerov1.BackupStorageLocationStatus{
							Phase: velerov1.BackupStorageLocationPhaseUnavailable,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Backup Storage Location default",
					Message: "Backup Storage Location [default] is in phase Unavailable",
					IsWarn:  true,
				},
				{
					Title:   "At least 1 Backup Storage Location configured",
					Message: "Found 1 configured backup storage locations, but none are available",
					IsWarn:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeBackupStorageLocations(tt.args.backupStorageLocations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeBackupStorageLocations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_DeleteBackupRequests(t *testing.T) {
	type args struct {
		deleteBackupRequests []*velerov1.DeleteBackupRequest
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no backup delete requests",
			args: args{
				deleteBackupRequests: []*velerov1.DeleteBackupRequest{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "backup delete requests completed",
			args: args{
				deleteBackupRequests: []*velerov1.DeleteBackupRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "observability-backup-20210308150016",
							Namespace: "velero",
						},
						Spec: velerov1.DeleteBackupRequestSpec{
							BackupName: "observability-backup",
						},
						Status: velerov1.DeleteBackupRequestStatus{
							Phase: velerov1.DeleteBackupRequestPhaseProcessed,
						},
					},
				},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "backup delete requests summarize in progress",
			args: args{
				deleteBackupRequests: []*velerov1.DeleteBackupRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "observability-backup-20210308150016",
							Namespace: "velero",
						},
						Spec: velerov1.DeleteBackupRequestSpec{
							BackupName: "observability-backup",
						},
						Status: velerov1.DeleteBackupRequestStatus{
							Phase: velerov1.DeleteBackupRequestPhaseInProgress,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Delete Backup Requests summary",
					Message: "Found 1 delete backup requests in progress",
					IsWarn:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeDeleteBackupRequests(tt.args.deleteBackupRequests); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeDeleteBackupRequests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_PodVolumeBackups(t *testing.T) {
	type args struct {
		podVolumeBackups []*velerov1.PodVolumeBackup
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no pod volume backups",
			args: args{
				podVolumeBackups: []*velerov1.PodVolumeBackup{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "pod volume backups",
			args: args{
				podVolumeBackups: []*velerov1.PodVolumeBackup{
					{
						Spec: velerov1.PodVolumeBackupSpec{
							Node: "test-node-1",
							Pod: corev1.ObjectReference{
								Kind:      "Pod",
								Name:      "kotsadm-76ddbc96c4-fsr88",
								Namespace: "default",
							},
							Volume:                "backup",
							BackupStorageLocation: "default",
						},
						Status: velerov1.PodVolumeBackupStatus{
							Phase: velerov1.PodVolumeBackupPhaseCompleted,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Pod Volume Backups",
					Message: "Found 1 pod volume backups",
					IsPass:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzePodVolumeBackups(tt.args.podVolumeBackups); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzePodVolumeBackups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_PodVolumeRestores(t *testing.T) {
	type args struct {
		podVolumeRestores []*velerov1.PodVolumeRestore
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no pod volume restores",
			args: args{
				podVolumeRestores: []*velerov1.PodVolumeRestore{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "pod volume restores - no failures",
			args: args{
				podVolumeRestores: []*velerov1.PodVolumeRestore{
					{
						Spec: velerov1.PodVolumeRestoreSpec{
							Pod: corev1.ObjectReference{
								Kind:      "Pod",
								Name:      "kotsadm-76ddbc96c4-fsr88",
								Namespace: "default",
							},
							Volume:                "backup",
							BackupStorageLocation: "default",
						},
						Status: velerov1.PodVolumeRestoreStatus{
							Phase: velerov1.PodVolumeRestorePhaseCompleted,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Pod Volume Restores",
					Message: "Found 1 pod volume restores",
					IsPass:  true,
				},
			},
		},
		{
			name: "pod volume restores - failures",
			args: args{
				podVolumeRestores: []*velerov1.PodVolumeRestore{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "observability-backup-20210308150016",
							Namespace: "velero",
						},
						Spec: velerov1.PodVolumeRestoreSpec{
							Pod: corev1.ObjectReference{
								Kind:      "Pod",
								Name:      "kotsadm-76ddbc96c4-fsr88",
								Namespace: "default",
							},
							Volume:                "backup",
							BackupStorageLocation: "default",
						},
						Status: velerov1.PodVolumeRestoreStatus{
							Phase: velerov1.PodVolumeRestorePhaseFailed,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Pod Volume Restore observability-backup-20210308150016",
					Message: "Pod Volume Restore observability-backup-20210308150016 phase is Failed",
					IsFail:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzePodVolumeRestores(tt.args.podVolumeRestores); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzePodVolumeRestores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_Restores(t *testing.T) {
	type args struct {
		restores []*velerov1.Restore
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no restores",
			args: args{
				restores: []*velerov1.Restore{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "restores completed",
			args: args{
				restores: []*velerov1.Restore{
					{
						Spec: velerov1.RestoreSpec{
							BackupName: "observability-backup",
						},
						Status: velerov1.RestoreStatus{
							Phase: velerov1.RestorePhaseCompleted,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero Restores",
					Message: "Found 1 restores",
					IsPass:  true,
				},
			},
		},
		{
			name: "restores - failures",
			args: args{
				restores: []*velerov1.Restore{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "observability-backup-20210308150016",
							Namespace: "velero",
						},
						Spec: velerov1.RestoreSpec{
							BackupName: "observability-backup",
						},
						Status: velerov1.RestoreStatus{
							Phase: velerov1.RestorePhaseWaitingForPluginOperationsPartiallyFailed,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Restore observability-backup-20210308150016",
					Message: "Restore observability-backup-20210308150016 phase is WaitingForPluginOperationsPartiallyFailed",
					IsFail:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeRestores(tt.args.restores); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeRestores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_Schedules(t *testing.T) {
	type args struct {
		schedules []*velerov1.Schedule
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no schedules",
			args: args{
				schedules: []*velerov1.Schedule{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "schedules configured",
			args: args{
				schedules: []*velerov1.Schedule{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "daily-backup",
							Namespace: "velero",
						},
						Spec: velerov1.ScheduleSpec{
							Schedule: "0 0 * * *",
							Template: velerov1.BackupSpec{
								StorageLocation: "default",
								IncludedNamespaces: []string{
									"default",
								},
								IncludedResources: []string{
									"*",
								},
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "velero",
									},
								},
							},
						},
						Status: velerov1.ScheduleStatus{
							Phase: velerov1.SchedulePhaseEnabled,
							LastBackup: &metav1.Time{
								Time: time.Date(2023, 3, 8, 15, 0, 16, 0, time.UTC),
							},
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero Schedules",
					Message: "Found 1 schedules",
					IsPass:  true,
				},
			},
		},
		{
			name: "schedules - failures",
			args: args{
				schedules: []*velerov1.Schedule{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "daily-backup",
							Namespace: "velero",
						},
						Spec: velerov1.ScheduleSpec{
							Schedule: "0 0 * * *",
							Template: velerov1.BackupSpec{
								StorageLocation: "default",
								IncludedNamespaces: []string{
									"default",
								},
								IncludedResources: []string{
									"*",
								},
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "velero",
									},
								},
							},
						},
						Status: velerov1.ScheduleStatus{
							Phase: velerov1.SchedulePhaseFailedValidation,
							LastBackup: &metav1.Time{
								Time: time.Date(2023, 3, 8, 15, 0, 16, 0, time.UTC),
							},
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Schedule daily-backup",
					Message: "Schedule daily-backup phase is FailedValidation",
					IsFail:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeSchedules(tt.args.schedules); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeSchedules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_VolumeSnapshotLocations(t *testing.T) {
	type args struct {
		volumeSnapshotLocations []*velerov1.VolumeSnapshotLocation
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no volume snapshot locations",
			args: args{
				volumeSnapshotLocations: []*velerov1.VolumeSnapshotLocation{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "volume snapshot locations configured",
			args: args{
				volumeSnapshotLocations: []*velerov1.VolumeSnapshotLocation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: "velero",
						},
						Spec: velerov1.VolumeSnapshotLocationSpec{
							Provider: "aws",
							Config: map[string]string{
								"region": "us-east-1",
							},
						},
						Status: velerov1.VolumeSnapshotLocationStatus{
							Phase: velerov1.VolumeSnapshotLocationPhaseAvailable,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero Volume Snapshot Locations",
					Message: "Found 1 volume snapshot locations",
					IsPass:  true,
				},
			},
		},
		{
			name: "volume snapshot locations - failures",
			args: args{
				volumeSnapshotLocations: []*velerov1.VolumeSnapshotLocation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: "velero",
						},
						Spec: velerov1.VolumeSnapshotLocationSpec{
							Provider: "aws",
							Config: map[string]string{
								"region": "us-east-1",
							},
						},
						Status: velerov1.VolumeSnapshotLocationStatus{
							Phase: velerov1.VolumeSnapshotLocationPhaseUnavailable,
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Volume Snapshot Location default",
					Message: "Volume Snapshot Location default phase is Unavailable",
					IsFail:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeVolumeSnapshotLocations(tt.args.volumeSnapshotLocations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeVolumeSnapshotLocations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_Logs(t *testing.T) {
	type args struct {
		logs map[string][]byte
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no logs",
			args: args{
				logs: map[string][]byte{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "logs - no errors",
			args: args{
				logs: map[string][]byte{
					"node-agent-m6n9j": []byte("level=info msg=... backup=velero/sample-app controller=podvolumebacku"),
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero Logs analysis",
					Message: "Found 1 log files",
					IsPass:  true,
				},
			},
		},
		{
			name: "logs - errors",
			args: args{
				logs: map[string][]byte{
					"node-agent-m6n9j": []byte("level=error msg=... backup=velero/sample-app controller=podvolumebacku"),
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Velero logs for pod [node-agent]",
					Message: "Found error|panic|fatal in node-agent* pod log file(s)",
					IsWarn:  true,
				},
				{
					Title:   "Velero Logs analysis",
					Message: "Found 1 log files",
					IsPass:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeLogs(tt.args.logs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeVelero_Results(t *testing.T) {
	type args struct {
		results []*AnalyzeResult
	}
	tests := []struct {
		name string
		args args
		want []*AnalyzeResult
	}{
		{
			name: "no results",
			args: args{
				results: []*AnalyzeResult{},
			},
			want: []*AnalyzeResult{},
		},
		{
			name: "results - pass",
			args: args{
				results: []*AnalyzeResult{
					{
						Title:   "random Velero CRD check",
						IsPass:  true,
						Message: "CRD status is healthy",
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "random Velero CRD check",
					IsPass:  true,
					Message: "CRD status is healthy",
				},
				{
					Title:   "Velero Status",
					IsPass:  true,
					Message: "Velero setup is healthy",
				},
			},
		},
		{
			name: "results - fail",
			args: args{
				results: []*AnalyzeResult{
					{
						Title:   "random Velero CRD check failure",
						IsFail:  true,
						Message: "CRD status - Failed",
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "random Velero CRD check failure",
					IsFail:  true,
					Message: "CRD status - Failed",
				},
				{
					Title:   "Velero Status",
					IsWarn:  true,
					Message: "Velero setup is not entirely healthy",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateResults(tt.args.results); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("aggregateResults() = %v, want %v", got, tt.want)
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Logf("\nGot: %s\nWant: %s", gotJSON, wantJSON)
			}
		})
	}
}

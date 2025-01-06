package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	appsV1 "k8s.io/api/apps/v1"

	restic_types "github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	"golang.org/x/mod/semver"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

type AnalyzeVelero struct {
	analyzer *troubleshootv1beta2.VeleroAnalyze
}

func (a *AnalyzeVelero) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = "Velero"
	}

	return title
}

func (a *AnalyzeVelero) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeVelero) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	results, err := a.veleroStatus(a.analyzer, getFile, findFiles)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return results, nil
}

func (a *AnalyzeVelero) veleroStatus(analyzer *troubleshootv1beta2.VeleroAnalyze, getFileContents getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	results := []*AnalyzeResult{}

	oldVeleroRepoType := false
	veleroVersion, err := getVeleroVersion(excludeFiles, findFiles)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to find velero deployment")
	}

	// Check if the version string is valid erer
	if !semver.IsValid(veleroVersion) {
		return nil, errors.Errorf("Invalid velero semver: %s", veleroVersion)
	}

	// check if veleroVersion is less than 1.10.x
	compareResult := semver.Compare(veleroVersion, "1.10.0")
	if compareResult < 0 {
		oldVeleroRepoType = true
	}

	// default to only the most recent Backup and Restore object if not specified in the analyzer spec
	backupCount := analyzer.BackupsCount
	if backupCount <= 0 {
		backupCount = 1
	}

	restoreCount := analyzer.RestoresCount
	if restoreCount <= 0 {
		restoreCount = 1
	}

	if oldVeleroRepoType == true {
		// old velero (v1.9.x) has a BackupRepositoryTypeRestic
		// get resticrepositories.velero.io
		resticRepositoriesDir := GetVeleroResticRepositoriesDirectory()
		resticRepositoriesGlob := filepath.Join(resticRepositoriesDir, "*.json")
		resticRepositoriesJson, err := findFiles(resticRepositoriesGlob, excludeFiles)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find velero restic repositories files under %s", resticRepositoriesDir)
		}
		resticRepositories := []*restic_types.ResticRepository{}
		for key, resticRepositoryJson := range resticRepositoriesJson {
			var resticRepositoryArray []*restic_types.ResticRepository
			err := json.Unmarshal(resticRepositoryJson, &resticRepositories)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal restic repository json from %s", key)
			}
			resticRepositories = append(resticRepositories, resticRepositoryArray...)
		}
		results = append(results, analyzeResticRepositories(resticRepositories)...)

	} else {

		// velerov1.Version
		// get backuprepositories.velero.io
		backupRepositoriesDir := GetVeleroBackupRepositoriesDirectory()
		backupRepositoriesGlob := filepath.Join(backupRepositoriesDir, "*.json")
		backupRepositoriesJson, err := findFiles(backupRepositoriesGlob, excludeFiles)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find velero backup repositories files under %s", backupRepositoriesDir)
		}
		backupRepositories := []*velerov1.BackupRepository{}
		for key, backupRepositoryJson := range backupRepositoriesJson {
			var backupRepositoryArray []*velerov1.BackupRepository
			err := json.Unmarshal(backupRepositoryJson, &backupRepositoryArray)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal backup repository json from %s", key)
			}
			backupRepositories = append(backupRepositories, backupRepositoryArray...)
		}
		results = append(results, analyzeBackupRepositories(backupRepositories)...)

	}

	// get backups.velero.io
	backupsDir := GetVeleroBackupsDirectory()
	backupsGlob := filepath.Join(backupsDir, "*.json")
	veleroJSONs, err := findFiles(backupsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero backup files")
	}
	backups := []*velerov1.Backup{}
	for _, veleroJSON := range veleroJSONs {
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read velero backup file %s", veleroJSON)
		}
		var veleroBackups []*velerov1.Backup
		err = json.Unmarshal(veleroJSON, &veleroBackups)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal velero backup file %s", veleroJSON)
		}
		backups = append(backups, veleroBackups...)
	}

	// get backupstoragelocations.velero.io
	backupStorageLocationsDir := GetVeleroBackupStorageLocationsDirectory()
	backupStorageLocationsGlob := filepath.Join(backupStorageLocationsDir, "*.json")
	backupStorageLocationsJson, err := findFiles(backupStorageLocationsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero backup storage locations files under %s", backupStorageLocationsDir)
	}
	backupStorageLocations := []*velerov1.BackupStorageLocation{}
	for key, backupStorageLocationJson := range backupStorageLocationsJson {
		var backupStorageLocationArray []*velerov1.BackupStorageLocation
		err := json.Unmarshal(backupStorageLocationJson, &backupStorageLocationArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal backup storage location json from %s", key)
		}
		backupStorageLocations = append(backupStorageLocations, backupStorageLocationArray...)
	}

	// get deletebackuprequests.velero.io
	deleteBackupRequestsDir := GetVeleroDeleteBackupRequestsDirectory()
	deleteBackupRequestsGlob := filepath.Join(deleteBackupRequestsDir, "*.json")
	deleteBackupRequestsJson, err := findFiles(deleteBackupRequestsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero delete backup requests files under %s", deleteBackupRequestsDir)
	}
	deleteBackupRequests := []*velerov1.DeleteBackupRequest{}
	for key, deleteBackupRequestJson := range deleteBackupRequestsJson {
		var deleteBackupRequestArray []*velerov1.DeleteBackupRequest
		err := json.Unmarshal(deleteBackupRequestJson, &deleteBackupRequestArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal delete backup request json from %s", key)
		}
		deleteBackupRequests = append(deleteBackupRequests, deleteBackupRequestArray...)
	}

	// get podvolumebackups.velero.io
	podVolumeBackupsDir := GetVeleroPodVolumeBackupsDirectory()
	podVolumeBackupsGlob := filepath.Join(podVolumeBackupsDir, "*.json")
	podVolumeBackupsJson, err := findFiles(podVolumeBackupsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero pod volume backups files under %s", podVolumeBackupsDir)
	}
	podVolumeBackups := []*velerov1.PodVolumeBackup{}
	for key, podVolumeBackupJson := range podVolumeBackupsJson {
		var podVolumeBackupArray []*velerov1.PodVolumeBackup
		err := json.Unmarshal(podVolumeBackupJson, &podVolumeBackupArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal pod volume backup json from %s", key)
		}
		podVolumeBackups = append(podVolumeBackups, podVolumeBackupArray...)
	}

	// get podvolumerestores.velero.io
	podVolumeRestoresDir := GetVeleroPodVolumeRestoresDirectory()
	podVolumeRestoresGlob := filepath.Join(podVolumeRestoresDir, "*.json")
	podVolumeRestoresJson, err := findFiles(podVolumeRestoresGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero pod volume restores files under %s", podVolumeRestoresDir)
	}
	podVolumeRestores := []*velerov1.PodVolumeRestore{}
	for key, podVolumeRestoreJson := range podVolumeRestoresJson {
		var podVolumeRestoreArray []*velerov1.PodVolumeRestore
		err := json.Unmarshal(podVolumeRestoreJson, &podVolumeRestoreArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal pod volume restore json from %s", key)
		}
		podVolumeRestores = append(podVolumeRestores, podVolumeRestoreArray...)
	}

	// get restores.velero.io
	restoresDir := GetVeleroRestoresDirectory()
	restoresGlob := filepath.Join(restoresDir, "*.json")
	restoresJson, err := findFiles(restoresGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero restores files under %s", restoresDir)
	}
	restores := []*velerov1.Restore{}
	for key, restoreJson := range restoresJson {
		var restoreArray []*velerov1.Restore
		err := json.Unmarshal(restoreJson, &restoreArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal restore json from %s", key)
		}
		restores = append(restores, restoreArray...)
	}

	// get schedules.velero.io
	schedulesDir := GetVeleroSchedulesDirectory()
	schedulesGlob := filepath.Join(schedulesDir, "*.json")
	schedulesJson, err := findFiles(schedulesGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero schedules files under %s", schedulesDir)
	}
	schedules := []*velerov1.Schedule{}
	for key, scheduleJson := range schedulesJson {
		var scheduleArray []*velerov1.Schedule
		err := json.Unmarshal(scheduleJson, &scheduleArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal schedule json from %s", key)
		}
		schedules = append(schedules, scheduleArray...)
	}

	// get serverstatusrequests.velero.io
	serverStatusRequestsDir := GetVeleroServerStatusRequestsDirectory()
	serverStatusRequestsGlob := filepath.Join(serverStatusRequestsDir, "*.json")
	serverStatusRequestsJson, err := findFiles(serverStatusRequestsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero server status requests files under %s", serverStatusRequestsDir)
	}
	serverStatusRequests := []*velerov1.ServerStatusRequest{}
	for key, serverStatusRequestJson := range serverStatusRequestsJson {
		var serverStatusRequestArray []*velerov1.ServerStatusRequest
		err := json.Unmarshal(serverStatusRequestJson, &serverStatusRequestArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal server status request json from %s", key)
		}
		serverStatusRequests = append(serverStatusRequests, serverStatusRequestArray...)
	}

	// get volumesnapshotlocations.velero.io
	volumeSnapshotLocationsDir := GetVeleroVolumeSnapshotLocationsDirectory()
	volumeSnapshotLocationsGlob := filepath.Join(volumeSnapshotLocationsDir, "*.json")
	volumeSnapshotLocationsJson, err := findFiles(volumeSnapshotLocationsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero volume snapshot locations files under %s", volumeSnapshotLocationsDir)
	}
	volumeSnapshotLocations := []*velerov1.VolumeSnapshotLocation{}
	for key, volumeSnapshotLocationJson := range volumeSnapshotLocationsJson {
		var volumeSnapshotLocationArray []*velerov1.VolumeSnapshotLocation
		err := json.Unmarshal(volumeSnapshotLocationJson, &volumeSnapshotLocationArray)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal volume snapshot location json from %s", key)
		}
		volumeSnapshotLocations = append(volumeSnapshotLocations, volumeSnapshotLocationArray...)
	}

	logsDir := GetVeleroLogsDirectory()
	logsGlob := filepath.Join(logsDir, "node-agent*", "*.log")
	nodeAgentlogs, err := findFiles(logsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero logs files under %s", logsDir)
	}

	veleroLogsGlob := filepath.Join(logsDir, "velero*", "*.log")
	veleroLogs, err := findFiles(veleroLogsGlob, excludeFiles)

	results = append(results, analyzeLogs(nodeAgentlogs, "node-agent*")...)
	results = append(results, analyzeLogs(veleroLogs, "velero*")...)
	results = append(results, analyzeBackups(backups, backupCount)...)
	results = append(results, analyzeBackupStorageLocations(backupStorageLocations)...)
	results = append(results, analyzeDeleteBackupRequests(deleteBackupRequests)...)
	results = append(results, analyzePodVolumeBackups(podVolumeBackups)...)
	results = append(results, analyzePodVolumeRestores(podVolumeRestores)...)
	results = append(results, analyzeRestores(restores, restoreCount)...)
	results = append(results, analyzeSchedules(schedules)...)
	results = append(results, analyzeVolumeSnapshotLocations(volumeSnapshotLocations)...)

	return aggregateResults(results), nil
}

func analyzeBackupRepositories(backupRepositories []*velerov1.BackupRepository) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	readyCount := 0
	backupRepositoriesResult := &AnalyzeResult{
		Title: "At least 1 Backup Repository configured",
	}
	if len(backupRepositories) == 0 {
		backupRepositoriesResult.IsFail = true
		backupRepositoriesResult.Message = "No backup repositories configured"
	} else {
		for _, backupRepository := range backupRepositories {
			if backupRepository.Status.Phase != velerov1.BackupRepositoryPhaseReady {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Backup Repository %s", backupRepository.Name),
				}
				result.Message = fmt.Sprintf("Backup Repository [%s] is in phase %s", backupRepository.Name, backupRepository.Status.Phase)
				result.IsWarn = true
				results = append(results, result)
			} else {
				readyCount++
			}
		}
		if readyCount > 0 {
			backupRepositoriesResult.IsPass = true
			backupRepositoriesResult.Message = fmt.Sprintf("Found %d backup repositories configured and %d Ready", len(backupRepositories), readyCount)
		} else {
			backupRepositoriesResult.IsWarn = true
			backupRepositoriesResult.Message = fmt.Sprintf("Found %d configured backup repositories, but none are ready", len(backupRepositories))
		}
	}
	results = append(results, backupRepositoriesResult)
	return results

}

func analyzeResticRepositories(resticRepositories []*restic_types.ResticRepository) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	readyCount := 0
	resticRepositoriesResult := &AnalyzeResult{
		Title: "At least 1 Restic Repository configured",
	}
	if len(resticRepositories) == 0 {
		resticRepositoriesResult.IsFail = true
		resticRepositoriesResult.Message = "No restic repositories configured"
	} else {
		for _, resticRepository := range resticRepositories {
			if resticRepository.Status.Phase != restic_types.ResticRepositoryPhaseReady {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Restic Repository %s", resticRepository.GetName()),
				}
				result.Message = fmt.Sprintf("Restic Repository [%s] is in phase %s", resticRepository.Name, resticRepository.Status.Phase)
				result.IsWarn = true
				results = append(results, result)
			} else {
				readyCount++
			}
		}
		if readyCount > 0 {
			resticRepositoriesResult.IsPass = true
			resticRepositoriesResult.Message = fmt.Sprintf("Found %d restic repositories configured and %d Ready", len(resticRepositories), readyCount)
		} else {
			resticRepositoriesResult.IsWarn = true
			resticRepositoriesResult.Message = fmt.Sprintf("Found %d configured restic repositories, but none are ready", len(resticRepositories))
		}
	}
	results = append(results, resticRepositoriesResult)
	return results
}

func analyzeBackups(backups []*velerov1.Backup, count int) []*AnalyzeResult {
	results := []*AnalyzeResult{}

	// Sort backups by StartTimestamp in descending order
	sort.SliceStable(backups, func(i, j int) bool {
		return backups[i].Status.StartTimestamp.After(backups[j].Status.StartTimestamp.Time)
	})

	// Limit to the most recent backupCount items
	if len(backups) > count {
		backups = backups[:count]
	}

	failedPhases := map[velerov1.BackupPhase]bool{
		velerov1.BackupPhaseFailed:           true,
		velerov1.BackupPhasePartiallyFailed:  true,
		velerov1.BackupPhaseFailedValidation: true,
	}

	for _, backup := range backups {

		if failedPhases[backup.Status.Phase] {
			result := &AnalyzeResult{
				Title: fmt.Sprintf("Backup %s", backup.Name),
			}
			result.IsFail = true
			result.Message = fmt.Sprintf("Backup %s phase is %s", backup.Name, backup.Status.Phase)
			results = append(results, result)

		}
	}
	if len(backups) > 0 {
		results = append(results, &AnalyzeResult{
			Title:   "Velero Backups",
			IsPass:  true,
			Message: fmt.Sprintf("Found %d backups", len(backups)),
		})
	}
	return results
}

func analyzeBackupStorageLocations(backupStorageLocations []*velerov1.BackupStorageLocation) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	availableCount := 0
	bslResult := &AnalyzeResult{
		Title: "At least 1 Backup Storage Location configured",
	}

	if len(backupStorageLocations) == 0 {
		bslResult.IsFail = true
		bslResult.Message = "No backup storage locations configured"
	} else {
		for _, backupStorageLocation := range backupStorageLocations {
			if backupStorageLocation.Status.Phase != velerov1.BackupStorageLocationPhaseAvailable {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Backup Storage Location %s", backupStorageLocation.Name),
				}
				result.Message = fmt.Sprintf("Backup Storage Location [%s] is in phase %s", backupStorageLocation.Name, backupStorageLocation.Status.Phase)
				result.IsWarn = true
				results = append(results, result)
			} else {
				availableCount++
			}
		}
		if availableCount > 0 {
			bslResult.IsPass = true
			bslResult.Message = fmt.Sprintf("Found %d backup storage locations configured and %d Available", len(backupStorageLocations), availableCount)
		} else {
			bslResult.IsWarn = true
			bslResult.Message = fmt.Sprintf("Found %d configured backup storage locations, but none are available", len(backupStorageLocations))
		}
	}
	results = append(results, bslResult)

	return results
}

func analyzeDeleteBackupRequests(deleteBackupRequests []*velerov1.DeleteBackupRequest) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	inProgressCount := 0
	if len(deleteBackupRequests) > 0 {
		for _, deleteBackupRequest := range deleteBackupRequests {
			if deleteBackupRequest.Status.Phase == velerov1.DeleteBackupRequestPhaseInProgress {
				inProgressCount++
			}
		}
		if inProgressCount > 0 {
			deleteBackupRequestsResult := &AnalyzeResult{
				Title: "Delete Backup Requests summary",
			}
			deleteBackupRequestsResult.IsWarn = true
			deleteBackupRequestsResult.Message = fmt.Sprintf("Found %d delete backup requests in progress", inProgressCount)
			results = append(results, deleteBackupRequestsResult)
		}
	}

	return results
}

func analyzePodVolumeBackups(podVolumeBackups []*velerov1.PodVolumeBackup) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	failures := 0
	if len(podVolumeBackups) > 0 {
		for _, podVolumeBackup := range podVolumeBackups {
			if podVolumeBackup.Status.Phase == velerov1.PodVolumeBackupPhaseFailed {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Pod Volume Backup %s", podVolumeBackup.Name),
				}
				result.IsFail = true
				result.Message = fmt.Sprintf("Pod Volume Backup %s phase is %s", podVolumeBackup.Name, podVolumeBackup.Status.Phase)
				results = append(results, result)
				failures++
			}
		}

		if failures == 0 {
			results = append(results, &AnalyzeResult{
				Title:   "Pod Volume Backups",
				IsPass:  true,
				Message: fmt.Sprintf("Found %d pod volume backups", len(podVolumeBackups)),
			})
		}
	}

	return results
}

func analyzePodVolumeRestores(podVolumeRestores []*velerov1.PodVolumeRestore) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	failures := 0

	if len(podVolumeRestores) > 0 {
		for _, podVolumeRestore := range podVolumeRestores {
			if podVolumeRestore.Status.Phase == velerov1.PodVolumeRestorePhaseFailed {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Pod Volume Restore %s", podVolumeRestore.Name),
				}
				result.IsFail = true
				result.Message = fmt.Sprintf("Pod Volume Restore %s phase is %s", podVolumeRestore.Name, podVolumeRestore.Status.Phase)
				results = append(results, result)
				failures++
			}
		}
		if failures == 0 {
			results = append(results, &AnalyzeResult{
				Title:   "Pod Volume Restores",
				IsPass:  true,
				Message: fmt.Sprintf("Found %d pod volume restores", len(podVolumeRestores)),
			})
		}
	}
	return results
}

func analyzeRestores(restores []*velerov1.Restore, count int) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	failures := 0

	// Sort restores by StartTimestamp in descending order
	sort.SliceStable(restores, func(i, j int) bool {
		return restores[i].Status.StartTimestamp.After(restores[j].Status.StartTimestamp.Time)
	})

	// Limit to the most recent restoreCount items
	if len(restores) > count {
		restores = restores[:count]
	}

	if len(restores) > 0 {

		failedPhases := map[velerov1.RestorePhase]bool{
			velerov1.RestorePhaseFailed:           true,
			velerov1.RestorePhasePartiallyFailed:  true,
			velerov1.RestorePhaseFailedValidation: true,
		}

		failureReasons := []string{
			"found a restore with status \"InProgress\" during the server starting, mark it as \"Failed\"",
		}

		for _, restore := range restores {
			if failedPhases[restore.Status.Phase] {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Restore %s", restore.Name),
				}
				result.IsFail = true
				result.Message = fmt.Sprintf("Restore %s phase is %s", restore.Name, restore.Status.Phase)
				results = append(results, result)
				failures++
			}
		}
		if failures == 0 {
			results = append(results, &AnalyzeResult{
				Title:   "Velero Restores",
				IsPass:  true,
				Message: fmt.Sprintf("Found %d restores", len(restores)),
			})
		}
	}

	return results
}

func analyzeSchedules(schedules []*velerov1.Schedule) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	failures := 0
	if len(schedules) > 0 {
		for _, schedule := range schedules {
			if schedule.Status.Phase == velerov1.SchedulePhaseFailedValidation {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Schedule %s", schedule.Name),
				}
				result.IsFail = true
				result.Message = fmt.Sprintf("Schedule %s phase is %s", schedule.Name, schedule.Status.Phase)
				results = append(results, result)
				failures++
			}
		}
		if failures == 0 {
			results = append(results, &AnalyzeResult{
				Title:   "Velero Schedules",
				IsPass:  true,
				Message: fmt.Sprintf("Found %d schedules", len(schedules)),
			})
		}
	}
	return results
}

func analyzeVolumeSnapshotLocations(volumeSnapshotLocations []*velerov1.VolumeSnapshotLocation) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	failures := 0
	if len(volumeSnapshotLocations) > 0 {
		for _, volumeSnapshotLocation := range volumeSnapshotLocations {
			if volumeSnapshotLocation.Status.Phase == velerov1.VolumeSnapshotLocationPhaseUnavailable {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Volume Snapshot Location %s", volumeSnapshotLocation.Name),
				}
				result.IsFail = true
				result.Message = fmt.Sprintf("Volume Snapshot Location %s phase is %s", volumeSnapshotLocation.Name, volumeSnapshotLocation.Status.Phase)
				results = append(results, result)
				failures++
			}
		}
		if failures == 0 {
			results = append(results, &AnalyzeResult{
				Title:   "Velero Volume Snapshot Locations",
				IsPass:  true,
				Message: fmt.Sprintf("Found %d volume snapshot locations", len(volumeSnapshotLocations)),
			})
		}
	}

	return results
}

func analyzeLogs(logs map[string][]byte, kind string) []*AnalyzeResult {
	results := []*AnalyzeResult{}
	if len(logs) > 0 {
		for key, logBytes := range logs {
			logContent := string(logBytes)
			result := &AnalyzeResult{
				Title: fmt.Sprintf("Velero logs for pod [%s]", key),
			}
			if strings.Contains(logContent, "permission denied") {
				result.IsWarn = true
				result.Message = fmt.Sprintf("Found 'permission denied' in %s pod log file(s)", kind)
				results = append(results, result)
				continue
			}

			if strings.Contains(logContent, "error") || strings.Contains(logContent, "panic") || strings.Contains(logContent, "fatal") {
				result.IsWarn = true
				result.Message = fmt.Sprintf("Found error|panic|fatal in %s pod log file(s)", kind)
				results = append(results, result)
			}
		}

		results = append(results, &AnalyzeResult{
			Title:   fmt.Sprintf("Velero Logs analysis for kind [%s]", kind),
			IsPass:  true,
			Message: fmt.Sprintf("Found %d log files", len(logs)),
		})
	}
	return results
}

func aggregateResults(results []*AnalyzeResult) []*AnalyzeResult {
	out := []*AnalyzeResult{}
	resultFailed := false
	for _, result := range results {
		if result.IsFail {
			resultFailed = true
		}
		out = append(out, result)
	}
	if len(results) > 0 {
		if resultFailed == false {
			out = append(out, &AnalyzeResult{
				Title:   "Velero Status",
				IsPass:  true,
				Message: "Velero setup is healthy",
			})
		}
		if resultFailed == true {
			out = append(out, &AnalyzeResult{
				Title:   "Velero Status",
				IsWarn:  true,
				Message: "Velero setup is not entirely healthy",
			})
		}
	}

	return out
}

func getVeleroVersion(excludedFiles []string, findFiles getChildCollectedFileContents) (string, error) {
	veleroDeploymentDir := "cluster-resources/deployments"
	veleroVersion := ""
	veleroDeploymentGlob := filepath.Join(veleroDeploymentDir, "velero.json")
	veleroDeploymentJson, err := findFiles(veleroDeploymentGlob, excludedFiles)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find Velero deployment")
	}
	if len(veleroDeploymentJson) == 0 {
		return "", errors.Errorf("could not find Velero deployment in %s", veleroDeploymentDir)
	}
	var deploymentList *appsV1.DeploymentList
	// should run only once
	for key, veleroDeploymentJsonBytes := range veleroDeploymentJson {
		err := json.Unmarshal(veleroDeploymentJsonBytes, &deploymentList)
		if err != nil {
			return "", errors.Wrapf(err, "failed to unmarshal Velero deployment json from %s", key)
		}
		break
	}
	if deploymentList == nil {
		return "", errors.Errorf("could not find Velero deployment")
	}
	for _, deployment := range deploymentList.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == "velero" {
				container_image := container.Image
				veleroVersion = strings.Split(container_image, ":")[1]
				return veleroVersion, nil
			}
		}
	}

	return "", errors.Errorf("could not find Velero container in deployment")
}

func GetVeleroBackupsDirectory() string {
	return "cluster-resources/custom-resources/backups.velero.io"
}

func GetVeleroBackupRepositoriesDirectory() string {
	return "cluster-resources/custom-resources/backuprepositories.velero.io"
}

func GetVeleroBackupStorageLocationsDirectory() string {
	return "cluster-resources/custom-resources/backupstoragelocations.velero.io"
}

func GetVeleroDeleteBackupRequestsDirectory() string {
	return "cluster-resources/custom-resources/deletebackuprequests.velero.io"
}

func GetVeleroDownloadRequestsDirectory() string {
	return "cluster-resources/custom-resources/downloadrequests.velero.io"
}

func GetVeleroLogsDirectory() string {
	return "velero/logs"
}

func GetVeleroPodVolumeBackupsDirectory() string {
	return "cluster-resources/custom-resources/podvolumebackups.velero.io"
}

func GetVeleroPodVolumeRestoresDirectory() string {
	return "cluster-resources/custom-resources/podvolumerestores.velero.io"
}

func GetVeleroRestoresDirectory() string {
	return "cluster-resources/custom-resources/restores.velero.io"
}

func GetVeleroSchedulesDirectory() string {
	return "cluster-resources/custom-resources/schedules.velero.io"
}

func GetVeleroServerStatusRequestsDirectory() string {
	return "cluster-resources/custom-resources/serverstatusrequests.velero.io"
}

func GetVeleroVolumeSnapshotLocationsDirectory() string {
	return "cluster-resources/custom-resources/volumesnapshotlocations.velero.io"
}

func GetVeleroResticRepositoriesDirectory() string {
	return "cluster-resources/custom-resources/resticrepositories.velero.io"
}

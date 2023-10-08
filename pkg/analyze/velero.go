package analyzer

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"gopkg.in/yaml.v2"
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
	ns := collect.DefaultVeleroNamespace
	if analyzer.Namespace != "" {
		ns = analyzer.Namespace
	}

	excludeFiles := []string{}

	// get backups.velero.io
	backupsDir := collect.GetVeleroBackupsDirectory(ns)
	backupsGlob := filepath.Join(backupsDir, "*")
	backupsYaml, err := findFiles(backupsGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero backups files under %s", backupsDir)
	}
	backups := []*velerov1.Backup{}
	for key, backupYaml := range backupsYaml {
		backup := &velerov1.Backup{}
		err := yaml.Unmarshal(backupYaml, backup)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal backup yaml from %s", key)
		}
		backups = append(backups, backup)
	}
	// fmt.Printf("\n..found %d backups\n", len(backups))

	// get backuprepositories.velero.io
	backupRpositoriesDir := collect.GetVeleroBackupRepositoriesDirectory(ns)
	backupRepositoriesGlob := filepath.Join(backupRpositoriesDir, "*")
	backupRepositoriesYaml, err := findFiles(backupRepositoriesGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find velero backup repositories files under %s", backupRpositoriesDir)
	}
	backupRepositories := []*velerov1.BackupRepository{}
	for key, backupRepositoryYaml := range backupRepositoriesYaml {
		backupRepository := &velerov1.BackupRepository{}
		err := yaml.Unmarshal(backupRepositoryYaml, backupRepository)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal backup repository yaml from %s", key)
		}
		backupRepositories = append(backupRepositories, backupRepository)
	}

	results := []*AnalyzeResult{}

	results = append(results, analyzeBackups(backups)...)

	// get restores.velero.io
	// restoresDir := collect.GetVeleroRestoresDirectory(ns)

	// return print backup files found
	// return nil, fmt.Errorf("found %d backups, %d backup repositories", len(backups), len(backupRepositories))
	results = append(results, analyzeBackupRepositories(backupRepositories)...)

	return aggregateResults(results), nil
}

func analyzeBackups(backups []*velerov1.Backup) []*AnalyzeResult {
	results := []*AnalyzeResult{}

	failedPhases := map[velerov1.BackupPhase]bool{
		velerov1.BackupPhaseFailed:                                    true,
		velerov1.BackupPhasePartiallyFailed:                           true,
		velerov1.BackupPhaseFailedValidation:                          true,
		velerov1.BackupPhaseFinalizingPartiallyFailed:                 true,
		velerov1.BackupPhaseWaitingForPluginOperationsPartiallyFailed: true,
	}

	for _, backup := range backups {

		if failedPhases[backup.Status.Phase] {
			result := &AnalyzeResult{
				Title: fmt.Sprintf("Backup %s", backup.Name),
			}
			result.IsFail = true
			// result.Strict = true
			result.Message = fmt.Sprintf("Backup %s phase is %s", backup.Name, backup.Status.Phase)
			results = append(results, result)

		}
		// else if backup.Status.Phase == velerov1.BackupPhaseCompleted {
		// 	result.IsPass = true
		// 	// result.Strict = true
		// } else {
		// 	// may indicate phases like:
		// 	// - velerov1.BackupPhaseWaitingForPluginOperations
		// 	// - velerov1.BackupPhaseFinalizing
		// 	result.IsWarn = true
		// }

	}

	results = append(results, &AnalyzeResult{
		Title:   "Velero Backups count",
		IsPass:  true,
		Message: fmt.Sprintf("Found %d backups", len(backups)),
	})

	return results
}

func analyzeBackupRepositories(backupRepositories []*velerov1.BackupRepository) []*AnalyzeResult {

	results := []*AnalyzeResult{}

	backupRepositoriesResult := &AnalyzeResult{
		Title: "At least 1 Velero Backup Repository configured",
	}
	if len(backupRepositories) == 0 {
		backupRepositoriesResult.IsFail = true
		backupRepositoriesResult.Message = "No backup repositories configured"
	} else {
		for _, backupRepository := range backupRepositories {

			if backupRepository.Status.Phase == velerov1.BackupRepositoryPhaseNotReady {
				result := &AnalyzeResult{
					Title: fmt.Sprintf("Backup Repository %s", backupRepository.Name),
				}
				result.Message = fmt.Sprintf("Backup Repository [%s] is in phase NotReady", backupRepository.Name)
				result.IsWarn = true
				results = append(results, result)
				// result.Strict = false
			}
		}
		backupRepositoriesResult.IsPass = true
		backupRepositoriesResult.Message = fmt.Sprintf("Found %d configured backup repositories", len(backupRepositories))
	}
	results = append(results, backupRepositoriesResult)

	return results

}

func aggregateResults(results []*AnalyzeResult) []*AnalyzeResult {
	out := []*AnalyzeResult{}
	resultPass := false
	for _, result := range results {
		if result.IsPass {
			resultPass = true
			// continue
		}
		out = append(out, result)
	}

	if resultPass && len(out) == 0 {
		out = append(out, &AnalyzeResult{
			Title:   "Velero Status",
			IsPass:  true,
			Message: "Backups and CRDs are healthy",
		})
	}

	return out
}

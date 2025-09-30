package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// HomebrewPackageManager detects if a binary was installed via Homebrew
type HomebrewPackageManager struct {
	formula string
}

var _ PackageManager = (*HomebrewPackageManager)(nil)

type homebrewInfoOutput struct {
	Installed []struct {
		Version     string `json:"version"`
		InstalledOn bool   `json:"installed_on_request"`
		LinkedKeg   string `json:"linked_keg"`
	} `json:"installed"`
}

// NewHomebrewPackageManager creates a new Homebrew package manager detector
func NewHomebrewPackageManager(formula string) PackageManager {
	return &HomebrewPackageManager{
		formula: formula,
	}
}

// Name returns the human-readable name of the package manager
func (h *HomebrewPackageManager) Name() string {
	return "Homebrew"
}

// IsInstalled checks if the formula is installed via Homebrew
func (h *HomebrewPackageManager) IsInstalled() (bool, error) {
	// First check if brew command exists
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		// No brew command found, definitely not installed via brew
		return false, nil
	}

	// Check if the formula is installed
	out, err := exec.Command(brewPath, "info", h.formula, "--json").Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// brew info with an invalid (not installed) package name returns an error
				return false, nil
			}
		}
		return false, err
	}

	var info []homebrewInfoOutput
	if err := json.Unmarshal(out, &info); err != nil {
		return false, err
	}

	if len(info) == 0 {
		return false, nil
	}

	// Check if the formula has any installed versions
	return len(info[0].Installed) > 0, nil
}

// UpgradeCommand returns the command to upgrade the package
func (h *HomebrewPackageManager) UpgradeCommand() string {
	return fmt.Sprintf("brew upgrade %s", h.formula)
}

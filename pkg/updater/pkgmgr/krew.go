package pkgmgr

import (
	"fmt"
	"os/exec"
	"strings"
)

// KrewPackageManager detects if a binary was installed via kubectl krew
type KrewPackageManager struct{
	pluginName string
}

var _ PackageManager = (*KrewPackageManager)(nil)

// NewKrewPackageManager creates a new Krew package manager detector
func NewKrewPackageManager(pluginName string) PackageManager {
	return &KrewPackageManager{
		pluginName: pluginName,
	}
}

// Name returns the human-readable name of the package manager
func (k *KrewPackageManager) Name() string {
	return "kubectl krew"
}

// IsInstalled checks if the plugin is installed via krew
func (k *KrewPackageManager) IsInstalled() (bool, error) {
	// First check if kubectl krew command exists
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return false, nil
	}

	// Check if krew plugin is available
	out, err := exec.Command("kubectl", "krew", "version").Output()
	if err != nil {
		// krew not installed
		return false, nil
	}

	if !strings.Contains(string(out), "krew") {
		return false, nil
	}

	// Check if the plugin is installed by listing installed plugins
	listOut, err := exec.Command("kubectl", "krew", "list").Output()
	if err != nil {
		return false, err
	}

	// Check if our plugin is in the installed list
	installedPlugins := strings.Split(string(listOut), "\n")
	for _, line := range installedPlugins {
		// Lines are in format: "PLUGIN VERSION"
		if strings.HasPrefix(strings.TrimSpace(line), k.pluginName+" ") || strings.TrimSpace(line) == k.pluginName {
			return true, nil
		}
	}

	return false, nil
}

// UpgradeCommand returns the command to upgrade the plugin
func (k *KrewPackageManager) UpgradeCommand() string {
	return fmt.Sprintf("kubectl krew upgrade %s", k.pluginName)
}


package pkgmgr

// PackageManager represents an external package manager that can manage the binary
type PackageManager interface {
	// IsInstalled returns true if the package/formula is installed via this package manager
	IsInstalled() (bool, error)
	// UpgradeCommand returns the command the user should run to upgrade
	UpgradeCommand() string
	// Name returns the human-readable name of the package manager
	Name() string
}
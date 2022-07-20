package convert

import (
	"os"
	"path/filepath"
)

// ValidateOutputPath takes an output file path and returns it as an absolute path.
// It returns an error if the absolute path cannot be determined or if the parent directory does not exist.
func ValidateOutputPath(outputPath string) (string, error) {
	outputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Dir(outputPath)); err != nil {
		return "", err
	}
	return outputPath, nil
}

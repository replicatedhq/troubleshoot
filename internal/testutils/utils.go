package testutils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func GetTestFixture(t *testing.T, path string) string {
	t.Helper()
	p := filepath.Join("../../testdata", path)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// FileDir returns the directory of the current source file.
func FileDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

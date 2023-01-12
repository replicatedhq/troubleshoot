package testutils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

// Generates a temporary filename
func TempFilename(prefix string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(randBytes)))
}

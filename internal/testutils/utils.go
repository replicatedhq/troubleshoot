package testutils

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

func CreateTestFile(t *testing.T, path string) {
	t.Helper()

	CreateTestFileWithData(t, path, "Garbage for "+path)
}

func CreateTestFileWithData(t *testing.T, path, data string) {
	t.Helper()

	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(data), 0644)
	require.NoError(t, err)
}

func LogJSON(t *testing.T, v interface{}) {
	t.Helper()

	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Log(v)
	} else {
		t.Log(string(b))
	}
}

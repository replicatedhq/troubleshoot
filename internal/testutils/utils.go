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

	b, err := os.ReadFile(TestFixtureFilePath(t, path))
	require.NoError(t, err)
	return string(b)
}

func TestFixtureFilePath(t *testing.T, path string) string {
	t.Helper()

	if !filepath.IsAbs(path) {
		p, err := filepath.Abs(filepath.Join(FileDir(), "../../testdata", path))
		require.NoError(t, err)
		return p
	} else {
		return path
	}
}

// FileDir returns the directory of this source file
func FileDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

// Generates a temporary filename
func TempFilename(prefix string) string {
	return filepath.Join(os.TempDir(), generateTempName(prefix))
}

func generateTempName(prefix string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(randBytes))
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

	t.Log(AsJSON(t, v))
}

func AsJSON(t *testing.T, v interface{}) string {
	t.Helper()

	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%#v", v)
	} else {
		return string(b)
	}
}

func ServeFromFilePath(t *testing.T, data string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), generateTempName("testfile"))
	CreateTestFileWithData(t, path, data)
	return path
}

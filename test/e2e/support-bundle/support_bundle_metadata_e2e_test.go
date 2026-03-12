package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var metadataSpecTemplate = `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: metadata-test
spec:
  collectors:
    - clusterResources:
        exclude: true
    - clusterInfo:
        exclude: true
    - supportBundleMetadata:
        namespace: $NAMESPACE
`

func TestSupportBundleMetadata(t *testing.T) {
	secretName := "replicated-support-metadata"
	expectedData := map[string]string{
		"appVersion":  "1.2.3",
		"environment": "staging",
		"clusterID":   "abc-123",
	}

	feature := features.New("Support Bundle Metadata Collector").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: c.Namespace(),
				},
				Data: make(map[string][]byte),
			}
			for k, v := range expectedData {
				secret.Data[k] = []byte(v)
			}

			client, err := c.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, secret); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("check metadata/cluster.json and metadata/user.json contents", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer

			namespace := c.Namespace()
			supportBundleName := "metadata-test"
			spec := strings.ReplaceAll(metadataSpecTemplate, "$NAMESPACE", namespace)
			specPath := filepath.Join(t.TempDir(), "supportBundleMetadata.yaml")

			err := os.WriteFile(specPath, []byte(spec), 0644)
			require.NoError(t, err)

			expectedUserMetadata := map[string]string{
				"contactEmail": "support@example.com",
				"ticketID":     "ISSUE-42",
			}

			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), specPath,
				"--interactive=false",
				fmt.Sprintf("-o=%s", supportBundleName),
				"--metadata=contactEmail=support@example.com",
				"--metadata=ticketID=ISSUE-42",
			)
			cmd.Stdout = &out
			err = cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.Remove(tarPath)
				if err != nil {
					t.Fatal("Error removing file:", err)
				}
			}()

			// Validate metadata/cluster.json from the secret
			clusterJSON, err := readFileFromTar(tarPath, fmt.Sprintf("%s/metadata/cluster.json", supportBundleName))
			require.NoError(t, err)

			var clusterResult map[string]string
			err = json.Unmarshal(clusterJSON, &clusterResult)
			require.NoError(t, err)

			assert.Equal(t, expectedData, clusterResult)

			// Validate metadata/user.json from the --metadata flag
			userJSON, err := readFileFromTar(tarPath, fmt.Sprintf("%s/metadata/user.json", supportBundleName))
			require.NoError(t, err)

			var userResult map[string]string
			err = json.Unmarshal(userJSON, &userResult)
			require.NoError(t, err)

			assert.Equal(t, expectedUserMetadata, userResult)

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}

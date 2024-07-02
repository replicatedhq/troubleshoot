package collect

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestGetImageAuthConfigFromData(t *testing.T) {
	tests := []struct {
		name             string
		imageName        string
		dockerConfigJSON string
		expectedUsername string
		expectedPassword string
		expectedError    bool
	}{
		{
			name:             "docker.io auth",
			imageName:        "docker.io/myimage",
			dockerConfigJSON: `{"auths":{"docker.io":{"auth":"username:password"}}}`,
			expectedUsername: "username",
			expectedPassword: "password",
			expectedError:    false,
		},
		{
			name:             "docker.io auth multi colon",
			imageName:        "docker.io/myimage",
			dockerConfigJSON: `{"auths":{"docker.io":{"auth":"user:name:pass:word"}}}`,
			expectedError:    true,
		},
		{
			name:             "gcr.io auth",
			imageName:        "gcr.io/myimage",
			dockerConfigJSON: `{"auths":{"gcr.io":{"username":"_json_key","password":"sa-key"}}}`,
			expectedUsername: "_json_key",
			expectedPassword: "sa-key",
			expectedError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			imageRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", test.imageName))
			assert.NoError(t, err)

			pullSecrets := &v1beta2.ImagePullSecrets{
				SecretType: "kubernetes.io/dockerconfigjson",
				Data: map[string]string{
					".dockerconfigjson": base64.StdEncoding.EncodeToString([]byte(test.dockerConfigJSON)),
				},
			}

			authConfig, err := getImageAuthConfigFromData(imageRef, pullSecrets)
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, authConfig)
			assert.Equal(t, test.expectedUsername, authConfig.username)
			assert.Equal(t, test.expectedPassword, authConfig.password)
		})
	}
}

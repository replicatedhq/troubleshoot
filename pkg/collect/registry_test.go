package collect

import (
	"fmt"
	"testing"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestGetImageAuthConfigFromData(t *testing.T) {
	imageRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", "docker.io/myimage"))
	assert.NoError(t, err)

	// {"auths":{"docker.io":{"auth":"auth","username":"username","password":"password"}}}
	pullSecrets := &v1beta2.ImagePullSecrets{
		SecretType: "kubernetes.io/dockerconfigjson",
		Data: map[string]string{
			".dockerconfigjson": "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImF1dGgiLCJ1c2VybmFtZSI6InVzZXJuYW1lIiwicGFzc3dvcmQiOiJwYXNzd29yZCJ9fX0=",
		},
	}

	authConfig, err := getImageAuthConfigFromData(imageRef, pullSecrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.NotNil(t, authConfig)
	assert.Equal(t, "username", authConfig.username)
	assert.Equal(t, "password", authConfig.password)
}

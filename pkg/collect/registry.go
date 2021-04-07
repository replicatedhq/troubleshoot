package collect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RegistryImage struct {
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

type RegistryInfo struct {
	Images map[string]RegistryImage `json:"images"`
}

func Registry(c *Collector, registryCollector *troubleshootv1beta2.RegistryImages) (map[string][]byte, error) {
	registryInfo := RegistryInfo{
		Images: map[string]RegistryImage{},
	}

	for _, image := range registryCollector.Images {
		exists, err := imageExists(c, registryCollector, image)
		if err != nil {
			registryInfo.Images[image] = RegistryImage{
				Error: err.Error(),
			}
		} else {
			registryInfo.Images[image] = RegistryImage{
				Exists: exists,
			}
		}
	}

	b, err := json.MarshalIndent(registryInfo, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := registryCollector.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	registryOutput := map[string][]byte{
		fmt.Sprintf("registry/%s.json", collectorName): b,
	}

	return registryOutput, nil
}

func imageExists(c *Collector, registryCollector *troubleshootv1beta2.RegistryImages, image string) (bool, error) {
	opts := []remote.Option{
		remote.WithPlatform(containerregistryv1.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		}),
	}

	imageRef, err := name.ParseReference(image)
	if err != nil {
		return false, errors.Wrapf(err, "parsing reference %s", image)
	}

	authConfig, err := getImageAuthConfig(c, registryCollector, imageRef)
	if err != nil {
		return false, errors.Wrap(err, "failed to get auth config")
	}

	if authConfig != nil {
		opts = append(opts, remote.WithAuth(authn.FromConfig(*authConfig)))
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		_, err = remote.Get(imageRef, opts...)
		if err == nil {
			return true, nil
		}

		if strings.Contains(err.Error(), "EOF") {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		transportErr, ok := err.(*transport.Error)
		if !ok {
			return false, errors.Wrap(err, "failed to get image manifest")
		}
		if transportErr.StatusCode == http.StatusNotFound {
			return false, nil
		}

		return false, errors.Wrap(err, "failed to get image manifest")
	}

	return false, errors.Wrap(lastErr, "failed to retry")
}

func getImageAuthConfig(c *Collector, registryCollector *troubleshootv1beta2.RegistryImages, imageRef name.Reference) (*authn.AuthConfig, error) {
	if registryCollector.ImagePullSecrets == nil {
		return nil, nil
	}

	if registryCollector.ImagePullSecrets.Data != nil {
		config, err := getImageAuthConfigFromData(c, imageRef, registryCollector.ImagePullSecrets)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get auth from data")
		}
		return config, nil
	}

	if registryCollector.ImagePullSecrets.Name != "" {
		namespace := registryCollector.Namespace
		if namespace == "" {
			namespace = c.Namespace
		}
		if namespace == "" {
			namespace = "default"
		}
		config, err := getImageAuthConfigFromSecret(c, imageRef, registryCollector.ImagePullSecrets, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get auth from secret")
		}
		return config, nil
	}

	return nil, errors.New("image pull secret spec is not valid")
}

func getImageAuthConfigFromData(c *Collector, imageRef name.Reference, pullSecrets *v1beta2.ImagePullSecrets) (*authn.AuthConfig, error) {
	if pullSecrets.SecretType != "kubernetes.io/dockerconfigjson" {
		return nil, errors.Errorf("secret type is not supported: %s", pullSecrets.SecretType)
	}

	configJsonBase64 := pullSecrets.Data[".dockerconfigjson"]
	registry := imageRef.Context().RegistryStr()

	configJson, err := base64.StdEncoding.DecodeString(configJsonBase64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode docker config string")
	}

	dockerCfgJSON := struct {
		Auths map[string]struct {
			Auth []byte `json:"auth"`
		} `json:"auths"`
	}{}

	err = json.Unmarshal([]byte(configJson), &dockerCfgJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config json")
	}

	auth, ok := dockerCfgJSON.Auths[registry]
	if !ok {
		// Suport a mix of public and private images
		return nil, nil
	}

	parts := strings.Split(string(auth.Auth), ":")
	if len(parts) != 2 {
		return nil, errors.Errorf("expected 2 parts in the string, but found %d", len(parts))
	}

	authConfig := authn.AuthConfig{
		Username: parts[0],
		Password: parts[1],
	}

	return &authConfig, nil
}

func getImageAuthConfigFromSecret(c *Collector, imageRef name.Reference, pullSecrets *v1beta2.ImagePullSecrets, namespace string) (*authn.AuthConfig, error) {
	ctx := context.Background()

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, pullSecrets.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secret")
	}

	foundSecrets := &v1beta2.ImagePullSecrets{
		Name:       secret.Name,
		SecretType: string(secret.Type),
		Data: map[string]string{
			".dockerconfigjson": base64.StdEncoding.EncodeToString(secret.Data[".dockerconfigjson"]),
		},
	}

	config, err := getImageAuthConfigFromData(c, imageRef, foundSecrets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get auth from secret data")
	}

	return config, nil
}

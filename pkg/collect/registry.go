package collect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	imagedocker "github.com/containers/image/v5/docker"
	dockerref "github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	registryv2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type RegistryImage struct {
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

type RegistryInfo struct {
	Images map[string]RegistryImage `json:"images"`
}

type registryAuthConfig struct {
	username string
	password string
}

type CollectRegistry struct {
	Collector    *troubleshootv1beta2.RegistryImages
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectRegistry) Title() string {
	return getCollectorName(c)
}

func (c *CollectRegistry) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRegistry) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	registryInfo := RegistryInfo{
		Images: map[string]RegistryImage{},
	}

	for _, image := range c.Collector.Images {
		exists, err := imageExists(c.Namespace, c.ClientConfig, c.Collector, image, 10*time.Second)
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

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("registry/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}

func imageExists(namespace string, clientConfig *rest.Config, registryCollector *troubleshootv1beta2.RegistryImages, image string, deadline time.Duration) (bool, error) {
	imageRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", image))
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse image name %s", image)
	}

	authConfig, err := getImageAuthConfig(namespace, clientConfig, registryCollector, imageRef)
	if err != nil {
		klog.Errorf("failed to get auth config: %v", err)
		return false, errors.Wrap(err, "failed to get auth config")
	}

	sysCtx := types.SystemContext{
		DockerDisableV1Ping:         true,
		DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
	}
	if authConfig != nil {
		sysCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: authConfig.username,
			Password: authConfig.password,
		}
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		err := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			remoteImage, err := imageRef.NewImage(ctx, &sysCtx)
			if err != nil {
				return err
			}
			remoteImage.Close()
			return nil
		}()
		if err == nil {
			klog.V(2).Infof("image %s exists", image)
			return true, nil
		}

		klog.Errorf("failed to get image %s: %v", image, err)

		// if this is a context timeout, stop here so we dont run this check for too long
		if errors.Is(err, context.DeadlineExceeded) {
			return false, errors.Wrap(err, "failed to get image manifest")
		}

		if strings.Contains(err.Error(), "no image found in manifest list for architecture") {
			// manifest was downloaded, but no matching architecture found in manifest
			// should this count as image does not exist?
			// this binary's architecture is not necessarily what will run in the cluster
			return true, nil
		}

		if isNotFound(err) {
			return false, nil
		}

		if strings.Contains(err.Error(), "EOF") {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		return false, errors.Wrap(err, "failed to get image manifest")
	}

	return false, errors.Wrap(lastErr, "failed to retry")
}

func getImageAuthConfig(namespace string, clientConfig *rest.Config, registryCollector *troubleshootv1beta2.RegistryImages, imageRef types.ImageReference) (*registryAuthConfig, error) {
	if registryCollector.ImagePullSecrets == nil {
		return nil, nil
	}

	if registryCollector.ImagePullSecrets.Data != nil {
		config, err := getImageAuthConfigFromData(imageRef, registryCollector.ImagePullSecrets)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get auth from data")
		}
		return config, nil
	}

	if registryCollector.ImagePullSecrets.Name != "" {
		collectorNamespace := registryCollector.Namespace
		if collectorNamespace == "" {
			collectorNamespace = namespace
		}
		if collectorNamespace == "" {
			collectorNamespace = "default"
		}
		config, err := getImageAuthConfigFromSecret(clientConfig, imageRef, registryCollector.ImagePullSecrets, collectorNamespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get auth from secret")
		}
		return config, nil
	}

	return nil, errors.New("image pull secret spec is not valid")
}

func getImageAuthConfigFromData(imageRef types.ImageReference, pullSecrets *v1beta2.ImagePullSecrets) (*registryAuthConfig, error) {
	if pullSecrets.SecretType != "kubernetes.io/dockerconfigjson" {
		return nil, errors.Errorf("secret type is not supported: %s", pullSecrets.SecretType)
	}

	configJsonBase64 := pullSecrets.Data[".dockerconfigjson"]
	registry := dockerref.Domain(imageRef.DockerReference())

	configJson, err := base64.StdEncoding.DecodeString(configJsonBase64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode docker config string")
	}

	dockerCfgJSON := struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
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

	// gcr.io auth uses username and password, e.g. username: _json_key, password: <sa_key>
	if auth.Username != "" && auth.Password != "" {
		return &registryAuthConfig{
			username: auth.Username,
			password: auth.Password,
		}, nil
	}

	// docker.io auth uses auth, e.g. auth: <base64_encoded_username_password>
	// username and password can't contain colon
	// at least according to https://github.com/docker/cli/blob/v27.0.3/cli/config/configfile/file.go#L247
	// fallback to not decode for compatibility
	authStr := auth.Auth
	decodedAuth, err := base64.StdEncoding.DecodeString(authStr)
	if err == nil {
		authStr = string(decodedAuth)
	}

	parts := strings.Split(authStr, ":")
	if len(parts) != 2 {
		return nil, errors.Errorf("expected 2 parts in the auth string, but found %d", len(parts))
	}

	authConfig := registryAuthConfig{
		username: parts[0],
		password: strings.Trim(parts[1], "\x00"),
	}

	return &authConfig, nil
}

func getImageAuthConfigFromSecret(clientConfig *rest.Config, imageRef types.ImageReference, pullSecrets *v1beta2.ImagePullSecrets, namespace string) (*registryAuthConfig, error) {
	ctx := context.Background()

	client, err := kubernetes.NewForConfig(clientConfig)
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

	config, err := getImageAuthConfigFromData(imageRef, foundSecrets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get auth from secret data")
	}

	return config, nil
}

func isNotFound(err error) bool {
	switch err := err.(type) {
	case errcode.Errors:
		for _, e := range err {
			if isNotFound(e) {
				return true
			}
		}
		return false
	case errcode.Error:
		return err.Message == registryv2.ErrorCodeManifestUnknown.Message()
	}

	// this type will cause panic when compared to error type
	if _, ok := err.(imagedocker.ErrUnauthorizedForCredentials); ok {
		return false
	}

	cause := errors.Cause(err)
	if cause, ok := cause.(error); ok {
		if cause == err {
			return false
		}
	}

	return isNotFound(cause)
}

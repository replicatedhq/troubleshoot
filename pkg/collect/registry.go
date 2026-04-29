package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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
	Collector    *v1beta2.RegistryImages
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

func imageExists(namespace string, clientConfig *rest.Config, registryCollector *v1beta2.RegistryImages, image string, deadline time.Duration) (bool, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse image name %s", image)
	}

	authConfig, err := getImageAuthConfig(namespace, clientConfig, registryCollector, ref)
	if err != nil {
		klog.Errorf("failed to get auth config: %v", err)
		return false, errors.Wrap(err, "failed to get auth config")
	}

	insecureTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if deadline == 0 {
		deadline = 10 * time.Second
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		err := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()

			opts := []remote.Option{
				remote.WithContext(ctx),
				remote.WithTransport(insecureTransport),
			}
			if authConfig != nil {
				opts = append(opts, remote.WithAuth(&authn.Basic{
					Username: authConfig.username,
					Password: authConfig.password,
				}))
			}

			_, err := remote.Head(ref, opts...)
			return err
		}()
		if err == nil {
			klog.V(2).Infof("image %s exists", image)
			return true, nil
		}

		klog.Errorf("failed to get image %s: %v", image, err)

		if stderrors.Is(err, context.DeadlineExceeded) {
			return false, errors.Wrap(err, "failed to get image manifest")
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

func getImageAuthConfig(namespace string, clientConfig *rest.Config, registryCollector *v1beta2.RegistryImages, imageRef name.Reference) (*registryAuthConfig, error) {
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

func getImageAuthConfigFromData(imageRef name.Reference, pullSecrets *v1beta2.ImagePullSecrets) (*registryAuthConfig, error) {
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
	// go-containerregistry normalizes "docker.io" to "index.docker.io"
	// (name.DefaultRegistry). dockerconfigjson files in the wild often
	// key on "docker.io", so fall back to that for Hub auth.
	if !ok && registry == name.DefaultRegistry {
		auth, ok = dockerCfgJSON.Auths["docker.io"]
	}
	if !ok {
		// Support a mix of public and private images
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

func getImageAuthConfigFromSecret(clientConfig *rest.Config, imageRef name.Reference, pullSecrets *v1beta2.ImagePullSecrets, namespace string) (*registryAuthConfig, error) {
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
	var terr *transport.Error
	if stderrors.As(err, &terr) {
		return terr.StatusCode == http.StatusNotFound
	}
	return false
}

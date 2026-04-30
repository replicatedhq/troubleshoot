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
	imageRef, err := parseImageRef(image)
	if err != nil {
		return false, err
	}

	authConfig, err := getImageAuthConfig(namespace, clientConfig, registryCollector, imageRef)
	if err != nil {
		klog.Errorf("failed to get auth config: %v", err)
		return false, errors.Wrap(err, "failed to get auth config")
	}

	return imageExistsWithAuth(authConfig, imageRef, image, deadline)
}

func parseImageRef(image string) (name.Reference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image name %s", image)
	}
	return ref, nil
}

// imageExistsWithAuth checks if an image exists in a registry using optional auth credentials.
// authConfig may be nil for ambient credentials (e.g. ~/.docker/config.json).
// This is the shared core used by both the cluster-level and host-level registry collectors.
func imageExistsWithAuth(authConfig *registryAuthConfig, ref name.Reference, image string, deadline time.Duration) (bool, error) {
	// remote.DefaultTransport includes Proxy (HTTP_PROXY/HTTPS_PROXY), dial/TLS
	// timeouts, and keepalive; clone it so InsecureSkipVerify does not drop those.
	defaultTR, ok := remote.DefaultTransport.(*http.Transport)
	if !ok {
		return false, errors.New("remote.DefaultTransport is not *http.Transport")
	}
	insecureTransport := defaultTR.Clone()
	insecureTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec

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
			} else {
				opts = append(opts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
			}

			// Use Get (not Head) so 404 responses include a JSON body; the registry
			// API encodes MANIFEST_UNKNOWN vs NAME_UNKNOWN there, which we need to
			// distinguish. Head 404s typically have no body, so *transport.Error has
			// empty Errors and we cannot classify the failure.
			//
			// Get fetches the manifest (or list/index) for the tag or digest and does
			// not pick a per-platform child image, so this checks presence only, not
			// whether the image runs on a given architecture.
			_, err := remote.Get(ref, opts...)
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

func getImageAuthConfig(namespace string, clientConfig *rest.Config, registryCollector *troubleshootv1beta2.RegistryImages, imageRef name.Reference) (*registryAuthConfig, error) {
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
	// (name.DefaultRegistry); many dockerconfigjson files key on "docker.io"
	// instead. Fall back to the alias so existing user secrets keep working.
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

// isNotFound returns true only when the registry reports MANIFEST_UNKNOWN: the
// repository exists but the tag or digest has no manifest. A 404 with
// NAME_UNKNOWN (repository missing) is not "not found" in that sense; callers
// should see the error to diagnose a wrong image path. Unstructured 404s
// (e.g. empty body on HEAD) are not treated as a known-missing image.
func isNotFound(err error) bool {
	var terr *transport.Error
	if !stderrors.As(err, &terr) || terr.StatusCode != http.StatusNotFound {
		return false
	}
	if len(terr.Errors) == 0 {
		return false
	}
	for _, d := range terr.Errors {
		if d.Code == transport.NameUnknownErrorCode {
			return false
		}
	}
	for _, d := range terr.Errors {
		if d.Code == transport.ManifestUnknownErrorCode {
			return true
		}
	}
	return false
}

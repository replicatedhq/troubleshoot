package oci

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	credentials "github.com/oras-project/oras-credentials-go"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"k8s.io/klog/v2"
	"oras.land/oras-go/pkg/auth"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
	"oras.land/oras-go/pkg/registry"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	HelmCredentialsFileBasename = ".config/helm/registry/config.json"
)

var (
	ErrNoRelease = errors.New("no release found")
)

func PullPreflightFromOCI(uri string) ([]byte, error) {
	return pullFromOCI(context.Background(), uri, "replicated.preflight.spec", "replicated-preflight")
}

func PullSupportBundleFromOCI(uri string) ([]byte, error) {
	return pullFromOCI(context.Background(), uri, "replicated.supportbundle.spec", "replicated-supportbundle")
}

// PullSpecsFromOCI pulls both the preflight and support bundle specs from the given URI
//
// The URI is expected to be the same as the one used to install your KOTS application
// Example oci://registry.replicated.com/app-slug/unstable will endup pulling
// preflights from "registry.replicated.com/app-slug/unstable/replicated-preflight:latest"
// and support bundles from "registry.replicated.com/app-slug/unstable/replicated-supportbundle:latest"
// Both images have their own media types created when publishing KOTS OCI image.
// NOTE: This only works with replicated registries for now and for KOTS applications only
func PullSpecsFromOCI(ctx context.Context, uri string) ([]string, error) {
	// TODOs (API is opinionated, but we should be able to support these):
	// - Pulling from generic OCI registries (not just replicated)
	// - Pulling from registries that require authentication
	// - Passing in a complete URI including tags and image name

	rawSpecs := []string{}

	// First try to pull the preflight spec
	rawPreflight, err := pullFromOCI(ctx, uri, "replicated.preflight.spec", "replicated-preflight")
	if err != nil {
		// Ignore "not found" error and continue fetching the support bundle spec
		if !errors.Is(err, ErrNoRelease) {
			return nil, err
		}
	} else {
		rawSpecs = append(rawSpecs, string(rawPreflight))
	}

	// Then try to pull the support bundle spec
	rawSupportBundle, err := pullFromOCI(ctx, uri, "replicated.supportbundle.spec", "replicated-supportbundle")
	// If we had found a preflight spec, do not return an error
	if err != nil && len(rawSpecs) == 0 {
		return nil, err
	}
	rawSpecs = append(rawSpecs, string(rawSupportBundle))

	return rawSpecs, nil
}

func pullFromOCI(ctx context.Context, uri string, mediaType string, imageName string) ([]byte, error) {
type (
	// Client works with OCI-compliant registries
	Client struct {
		debug       bool
		enableCache bool
		// path to repository config file e.g. ~/.docker/config.json
		credentialsFile  string
		out              io.Writer
		authorizer       *auth.Client
		credentialsStore credentials.Store
		httpClient       *http.Client
		plainHTTP        bool
	}

	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default client
	ClientOption func(*Client)
)

func pullFromOCI(uri string, mediaType string, imageName string) ([]byte, error) {
	// helm credentials
	helmCredentialsFile := filepath.Join(util.HomeDir(), HelmCredentialsFileBasename)
	dockerauthClient, err := dockerauth.NewClientWithDockerFallback(helmCredentialsFile)

	// 0. Create an OCI layout store
	store, err := oci.New("/tmp/oci-layout-root")
	if err != nil {
		return err
	}

	// 1. Connect to a remote repository
	ctx := context.Background()
	reg := "docker.io"
	repo, err := remote.NewRepository(reg + "/user/my-repo")
	if err != nil {
		return err
	}

	// 2. Get credentials from the docker credential store
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return err
	}

	// Prepare the auth client for the registry and credential store
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.DefaultCache,
		Credential: credentials.Credential(credStore), // Use the credential store
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to create auth client")
	}

	authClient := dockerauthClient

	headers := http.Header{}
	headers.Set("User-Agent", version.GetUserAgent())
	opts := []auth.ResolverOption{auth.WithResolverHeaders(headers)}
	resolver, err := authClient.ResolverWithOpts(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create resolver")
	}

	memoryStore := memory.Store{}
	allowedMediaTypes := []string{
		mediaType,
	}

	var descriptors, layers []ocispec.Descriptor
	registryStore := content.Registry{Resolver: resolver}

	parsedRef, err := parseURI(uri, imageName)
	if err != nil {
		return nil, err
	}

	klog.V(1).Infof("Pulling spec from %q OCI uri", parsedRef)

	manifest, err := oras.Copy(ctx, registryStore, parsedRef, memoryStore, "",
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(allowedMediaTypes),
		oras.WithLayerDescriptors(func(l []ocispec.Descriptor) {
			layers = l
		}))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrNoRelease
		}

		return nil, errors.Wrap(err, "failed to copy")
	}

	descriptors = append(descriptors, manifest)
	descriptors = append(descriptors, layers...)

	// expect 2 descriptors
	if len(descriptors) != 2 {
		return nil, fmt.Errorf("expected 2 descriptor, got %d", len(descriptors))
	}

	var matchingDescriptor *ocispec.Descriptor

	for _, descriptor := range descriptors {
		d := descriptor
		switch d.MediaType {
		case mediaType:
			matchingDescriptor = &d
		}
	}

	if matchingDescriptor == nil {
		return nil, fmt.Errorf("no descriptor found with media type: %s", mediaType)
	}

	_, matchingSpec, ok := memoryStore.Get(*matchingDescriptor)
	if !ok {
		return nil, fmt.Errorf("failed to get matching descriptor")
	}

	return matchingSpec, nil
}

func parseURI(in, imageName string) (string, error) {
	u, err := url.Parse(in)
	if err != nil {
		return "", err
	}

	// Always check the scheme. If more schemes need to be supported
	// we need to compare u.Scheme against a list of supported schemes.
	// url.Parse(raw) will not return an error if a scheme is not present.
	if u.Scheme != "oci" {
		return "", fmt.Errorf("%q is an invalid OCI registry scheme", u.Scheme)
	}

	// remove unnecessary bits (oci://, tags)
	uriParts := strings.Split(u.EscapedPath(), ":")

	tag := "latest"
	if len(uriParts) > 1 {
		tag = uriParts[1]
	}

	uri := fmt.Sprintf("%s%s/%s:%s", u.Host, uriParts[0], imageName, tag) // <host>:<port>/path/<imageName>:tag

	parsedRef, err := registry.ParseReference(uri)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse OCI uri reference")
	}

	return parsedRef.String(), nil
}

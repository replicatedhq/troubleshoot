package oci

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

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

func PullSpecsFromOCI(ctx context.Context, uri string) ([]string, error) {
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
	// helm credentials
	helmCredentialsFile := filepath.Join(util.HomeDir(), HelmCredentialsFileBasename)
	dockerauthClient, err := dockerauth.NewClientWithDockerFallback(helmCredentialsFile)
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

	// TODO: How do we handle "not found" cases?
	memoryStore := content.NewMemory()
	allowedMediaTypes := []string{
		mediaType,
	}

	var descriptors, layers []ocispec.Descriptor
	registryStore := content.Registry{Resolver: resolver}

	parsedRef, err := toRegistryRef(uri)
	if err != nil {
		return nil, err
	}
	klog.V(1).Infof("Pulling OCI image from %q", parsedRef.String())

	manifest, err := oras.Copy(ctx, registryStore, parsedRef.String(), memoryStore, "",
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

func toRegistryRef(raw string) (registry.Reference, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return registry.Reference{}, err
	}

	// Always check the scheme. If more schemes need to be supported
	// we need to compare u.Scheme against a list of supported schemes.
	// url.Parse(raw) will not return an error is a scheme is not present.
	if u.Scheme != "oci" {
		return registry.Reference{}, fmt.Errorf("%q is an invalid OCI registry scheme", u.Scheme)
	}

	parts := strings.Split(u.EscapedPath(), ":")
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}
	// remove the oci://
	uri := fmt.Sprintf("%s%s:%s", u.Host, parts[0], tag)
	return registry.ParseReference(uri)
}

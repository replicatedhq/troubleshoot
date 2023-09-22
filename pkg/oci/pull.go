package oci

import (
	"context"
	"fmt"
	"net/http"
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

// PullSpecsFromOCI pulls both the preflight and support bundle specs from the given URI
//
// The URI is expected to be the same as the one used to install your KOTS application
// Example oci://registry.replicated.com/thanos-reloaded/unstable will endup pulling
// preflights from "registry.replicated.com/thanos-reloaded/unstable/replicated-preflight:latest"
// and support bundles from "registry.replicated.com/thanos-reloaded/unstable/replicated-supportbundle:latest"
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

	memoryStore := content.NewMemory()
	allowedMediaTypes := []string{
		mediaType,
	}

	var descriptors, layers []ocispec.Descriptor
	registryStore := content.Registry{Resolver: resolver}

	// remove the oci://
	uri = strings.TrimPrefix(uri, "oci://")

	uriParts := strings.Split(uri, ":")
	uri = fmt.Sprintf("%s/%s", uriParts[0], imageName)

	if len(uriParts) > 1 {
		uri = fmt.Sprintf("%s:%s", uri, uriParts[1])
	} else {
		uri = fmt.Sprintf("%s:latest", uri)
	}

	parsedRef, err := registry.ParseReference(uri)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse reference")
	}

	klog.V(1).Infof("Pulling spec from %q OCI uri", parsedRef.String())

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

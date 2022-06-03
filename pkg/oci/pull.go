package oci

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"oras.land/oras-go/pkg/auth"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
	"oras.land/oras-go/pkg/registry"
)

const (
	HelmCredentialsFileBasename = "registry/config.json"
)

var (
	ErrNoRelease = errors.New("no release found")
)

func PullPreflightFromOCI(uri string) ([]byte, error) {
	return pullFromOCI(uri, "replicated.preflight.spec", "replicated-preflight")
}

func PullSupportBundleFromOCI(uri string) ([]byte, error) {
	return pullFromOCI(uri, "replicated.supportbundle.spec", "replicated-supportbundle")
}

func pullFromOCI(uri string, mediaType string, imageName string) ([]byte, error) {
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

	manifest, err := oras.Copy(context.TODO(), registryStore, parsedRef.String(), memoryStore, "",
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

	// expect 1 descriptor
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

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
	"github.com/replicatedhq/troubleshoot/pkg/oci/types"
	"github.com/replicatedhq/troubleshoot/pkg/version"
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

func PullPreflightFromOCI(uri string) (types.Layers, error) {
	layers := &PreflightLayers{}

	err := pullFromOCI(uri, "replicated-preflight", layers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pull OCI preflight")
	}

	return layers, nil
}

func PullSupportBundleFromOCI(uri string) (types.Layers, error) {
	layers := &SupportBundleLayers{}

	err := pullFromOCI(uri, "replicated-supportbundle", layers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pull OCI supportbundle")
	}

	return layers, nil
}

func pullFromOCI(uri string, imageName string, outputLayers types.Layers) error {
	// helm credentials
	helmCredentialsFile := filepath.Join(util.HomeDir(), HelmCredentialsFileBasename)
	dockerauthClient, err := dockerauth.NewClientWithDockerFallback(helmCredentialsFile)
	if err != nil {
		return errors.Wrap(err, "failed to create auth client")
	}

	authClient := dockerauthClient

	headers := http.Header{}
	headers.Set("User-Agent", version.GetUserAgent())
	opts := []auth.ResolverOption{auth.WithResolverHeaders(headers)}
	resolver, err := authClient.ResolverWithOpts(opts...)
	if err != nil {
		return errors.Wrap(err, "failed to create resolver")
	}

	memoryStore := content.NewMemory()

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
		return errors.Wrap(err, "failed to parse reference")
	}

	manifest, err := oras.Copy(context.TODO(), registryStore, parsedRef.String(), memoryStore, "",
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(outputLayers.GetAllowedMediaTypes()),
		oras.WithLayerDescriptors(func(l []ocispec.Descriptor) {
			layers = l
		}))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrNoRelease
		}

		return errors.Wrap(err, "failed to copy")
	}

	descriptors = append(descriptors, manifest)
	descriptors = append(descriptors, layers...)

	for _, descriptor := range descriptors {
		_, layerData, ok := memoryStore.Get(descriptor)
		if !ok {
			return fmt.Errorf("failed to get data for %s", descriptor.MediaType)
		}
		outputLayers.SetLayer(descriptor.MediaType, layerData)
	}

	if outputLayers.IsEmpty() {
		return fmt.Errorf("layers found with media type: %v", outputLayers.GetAllowedMediaTypes())
	}

	return nil
}

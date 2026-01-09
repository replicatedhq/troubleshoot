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
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
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
	// Parse the URI to get the repository reference
	ref, err := parseURI(uri, imageName)
	if err != nil {
		return nil, err
	}

	klog.V(1).Infof("Pulling spec from %q OCI uri", ref)

	// Create a repository instance
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create repository")
	}

	// Set up authentication with Docker credentials fallback
	helmCredentialsFile := filepath.Join(util.HomeDir(), HelmCredentialsFileBasename)
	storeOpts := credentials.StoreOptions{}

	// Try to load credentials from Helm config first, fall back to Docker config
	var credStore credentials.Store
	helmStore, helmErr := credentials.NewStore(helmCredentialsFile, storeOpts)
	if helmErr == nil {
		credStore = helmStore
	} else {
		// Fall back to Docker credentials if helm credentials are not available
		dockerStore, dockerErr := credentials.NewStoreFromDocker(storeOpts)
		if dockerErr != nil {
			return nil, errors.Wrap(dockerErr, "failed to create credential store")
		}
		credStore = dockerStore
	}

	// Configure the repository client with authentication and custom headers
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
		Header: http.Header{
			"User-Agent": []string{version.GetUserAgent()},
		},
	}

	// Create in-memory storage for the pulled content
	memoryStore := memory.New()

	// Track layers for filtering
	var layers []ocispec.Descriptor

	// Set up copy options to capture layer descriptors
	copyOpts := oras.CopyOptions{}
	copyOpts.CopyGraphOptions.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		// Filter by media type - only copy layers with the specified media type
		if desc.MediaType == mediaType {
			layers = append(layers, desc)
			return nil
		}
		// Allow manifest and other necessary descriptors
		if strings.Contains(desc.MediaType, "manifest") || strings.Contains(desc.MediaType, "config") {
			return nil
		}
		// Skip other media types
		return oras.SkipNode
	}

	// Copy from the repository to memory
	tag := repo.Reference.Reference
	if tag == "" {
		tag = "latest"
	}

	manifest, err := oras.Copy(ctx, repo, tag, memoryStore, tag, copyOpts)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "manifest unknown") {
			return nil, ErrNoRelease
		}
		return nil, errors.Wrap(err, "failed to copy")
	}

	descriptors := []ocispec.Descriptor{manifest}
	descriptors = append(descriptors, layers...)

	// expect 2 descriptors (manifest + one layer)
	if len(descriptors) != 2 {
		return nil, fmt.Errorf("expected 2 descriptors, got %d", len(descriptors))
	}

	var matchingDescriptor *ocispec.Descriptor

	for _, descriptor := range descriptors {
		d := descriptor
		if d.MediaType == mediaType {
			matchingDescriptor = &d
		}
	}

	if matchingDescriptor == nil {
		return nil, fmt.Errorf("no descriptor found with media type: %s", mediaType)
	}

	// Fetch the content from memory store
	reader, err := memoryStore.Fetch(ctx, *matchingDescriptor)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch matching descriptor")
	}
	defer reader.Close()

	// Read all content
	matchingSpec := make([]byte, matchingDescriptor.Size)
	if _, err := reader.Read(matchingSpec); err != nil {
		return nil, errors.Wrap(err, "failed to read content")
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

	// Format as: <host>:<port>/path/<imageName>:tag
	// The remote.NewRepository() function in v2 can handle this format directly
	uri := fmt.Sprintf("%s%s/%s:%s", u.Host, uriParts[0], imageName, tag)

	return uri, nil
}

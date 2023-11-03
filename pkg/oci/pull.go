package oci

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	// ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"oras.land/oras-go/v2/registry"

	// "github.com/replicatedhq/troubleshoot/pkg/version"
	// oras "oras.land/oras-go/v2"
	// "oras.land/oras-go/v2/content"
	// "oras.land/oras-go/v2/content/memory"
	// "oras.land/oras-go/v2/content/oci"
	// "oras.land/oras-go/v2/registry"

	// "oras.land/oras-go/v2/registry/remote/retry"
	credentials "github.com/oras-project/oras-credentials-go"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

var (
	ErrNoRelease = errors.New("no release found")
)

func PullPreflightFromOCI(uri string) ([]byte, error) {
	return pullImageSpecsOCI(context.Background(), uri, "replicated.preflight.spec", "replicated-preflight")
}

func PullSupportBundleFromOCI(uri string) ([]byte, error) {
	return pullImageSpecsOCI(context.Background(), uri, "replicated.supportbundle.spec", "replicated-supportbundle")
}

func pullImageSpecsOCI(ctx context.Context, uri string, mediaType string, imageName string) ([]byte, error) {

	// 0. Create an OCI layout store
	tempDir, err := os.MkdirTemp("", "oci-layout-root")
	if err != nil {
		return []byte{}, err
	}

	store, err := oci.New(tempDir)
	if err != nil {
		return []byte{}, err
	}

	// 1. Connect to a remote repository
	// ctx := context.Background()
	reg := "registry.replicated.com"
	repo, err := remote.NewRepository(reg + "/library/replicated")
	if err != nil {
		return []byte{}, err
	}

	// 2. Get credentials from the docker credential store
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return []byte{}, err
	}

	// Prepare the auth client for the registry and credential store
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.DefaultCache,
		Credential: credentials.Credential(credStore), // Use the credential store
	}

	// 3. Copy from the remote repository to the OCI layout store
	tag := "1.0.0-beta.10"
	manifestDescriptor, err := oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return []byte{}, err
	}

	fmt.Println("manifest pulled:", manifestDescriptor.Digest, manifestDescriptor.MediaType)

	// 3. Fetch from OCI layout store to verify
	fetched, err := content.FetchAll(ctx, store, manifestDescriptor)
	if err != nil {
		return []byte{}, err
	}
	fmt.Printf("manifest content:\n%s", fetched)
	return fetched, nil
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
	rawPreflight, err := pullImageSpecsOCI(ctx, uri, "replicated.preflight.spec", "replicated-preflight")
	if err != nil {
		// Ignore "not found" error and continue fetching the support bundle spec
		if !errors.Is(err, ErrNoRelease) {
			return nil, err
		}
	} else {
		rawSpecs = append(rawSpecs, string(rawPreflight))
	}

	// Then try to pull the support bundle spec
	rawSupportBundle, err := pullImageSpecsOCI(ctx, uri, "replicated.supportbundle.spec", "replicated-supportbundle")
	// If we had found a preflight spec, do not return an error
	if err != nil && len(rawSpecs) == 0 {
		return nil, err
	}
	rawSpecs = append(rawSpecs, string(rawSupportBundle))

	return rawSpecs, nil
}

// type (
// 	// Client works with OCI-compliant registries
// 	Client struct {
// 		debug       bool
// 		enableCache bool
// 		// path to repository config file e.g. ~/.docker/config.json
// 		credentialsFile  string
// 		out              io.Writer
// 		authorizer       *auth.Client
// 		credentialsStore credentials.Store
// 		httpClient       *http.Client
// 		plainHTTP        bool
// 	}

// 	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
// 	// used when creating a new default client
// 	ClientOption func(*Client)
// )

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

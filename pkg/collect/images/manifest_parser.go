package images

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// V2Manifest represents a Docker v2 or OCI manifest
type V2Manifest struct {
	SchemaVersion int         `json:"schemaVersion"`
	MediaType     string      `json:"mediaType"`
	Config        Descriptor  `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

// GetMediaType returns the manifest media type
func (m *V2Manifest) GetMediaType() string {
	return m.MediaType
}

// GetSchemaVersion returns the manifest schema version
func (m *V2Manifest) GetSchemaVersion() int {
	return m.SchemaVersion
}

// GetConfig returns the config descriptor
func (m *V2Manifest) GetConfig() Descriptor {
	return m.Config
}

// GetLayers returns the layer descriptors
func (m *V2Manifest) GetLayers() []Descriptor {
	return m.Layers
}

// GetPlatform returns platform information (not available in v2 manifests directly)
func (m *V2Manifest) GetPlatform() *Platform {
	return nil // Platform info is in the config blob for v2 manifests
}

// Marshal serializes the manifest to JSON
func (m *V2Manifest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// ManifestList represents a Docker manifest list or OCI image index
type ManifestList struct {
	SchemaVersion int                 `json:"schemaVersion"`
	MediaType     string              `json:"mediaType"`
	Manifests     []ManifestDescriptor `json:"manifests"`
}

// ManifestDescriptor represents a manifest in a manifest list
type ManifestDescriptor struct {
	Descriptor
	Platform *Platform `json:"platform,omitempty"`
}

// GetMediaType returns the manifest media type
func (m *ManifestList) GetMediaType() string {
	return m.MediaType
}

// GetSchemaVersion returns the manifest schema version
func (m *ManifestList) GetSchemaVersion() int {
	return m.SchemaVersion
}

// GetConfig returns the config descriptor (not applicable for manifest lists)
func (m *ManifestList) GetConfig() Descriptor {
	return Descriptor{} // Manifest lists don't have a single config
}

// GetLayers returns the layer descriptors (not applicable for manifest lists)
func (m *ManifestList) GetLayers() []Descriptor {
	return nil // Manifest lists contain manifests, not layers
}

// GetPlatform returns platform information (not applicable for manifest lists)
func (m *ManifestList) GetPlatform() *Platform {
	return nil // Manifest lists contain multiple platforms
}

// Marshal serializes the manifest to JSON
func (m *ManifestList) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// GetManifestForPlatform returns the best manifest for the given platform
func (m *ManifestList) GetManifestForPlatform(targetPlatform Platform) (*ManifestDescriptor, error) {
	if len(m.Manifests) == 0 {
		return nil, errors.New("manifest list is empty")
	}

	// First try exact match
	for _, manifest := range m.Manifests {
		if manifest.Platform != nil && 
		   manifest.Platform.Architecture == targetPlatform.Architecture &&
		   manifest.Platform.OS == targetPlatform.OS {
			if targetPlatform.Variant == "" || manifest.Platform.Variant == targetPlatform.Variant {
				return &manifest, nil
			}
		}
	}

	// Fallback to first linux/amd64 if available
	for _, manifest := range m.Manifests {
		if manifest.Platform != nil &&
		   manifest.Platform.OS == "linux" &&
		   manifest.Platform.Architecture == "amd64" {
			return &manifest, nil
		}
	}

	// Last resort: return first manifest
	return &m.Manifests[0], nil
}

// V1Manifest represents a Docker v1 manifest (legacy)
type V1Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	Tag           string `json:"tag"`
	Architecture  string `json:"architecture"`
	FsLayers      []struct {
		BlobSum string `json:"blobSum"`
	} `json:"fsLayers"`
	History []struct {
		V1Compatibility string `json:"v1Compatibility"`
	} `json:"history"`
}

// GetMediaType returns the manifest media type
func (m *V1Manifest) GetMediaType() string {
	return DockerManifestSchema1
}

// GetSchemaVersion returns the manifest schema version
func (m *V1Manifest) GetSchemaVersion() int {
	return m.SchemaVersion
}

// GetConfig returns the config descriptor (convert from v1 format)
func (m *V1Manifest) GetConfig() Descriptor {
	// v1 manifests don't have separate config blobs
	return Descriptor{}
}

// GetLayers returns the layer descriptors (convert from v1 format)
func (m *V1Manifest) GetLayers() []Descriptor {
	layers := make([]Descriptor, len(m.FsLayers))
	for i, layer := range m.FsLayers {
		layers[i] = Descriptor{
			Digest:    layer.BlobSum,
			MediaType: DockerImageLayer,
		}
	}
	return layers
}

// GetPlatform returns platform information
func (m *V1Manifest) GetPlatform() *Platform {
	return &Platform{
		Architecture: m.Architecture,
		OS:           "linux", // v1 manifests are typically Linux
	}
}

// Marshal serializes the manifest to JSON
func (m *V1Manifest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// ImageConfigBlob represents the image configuration blob
type ImageConfigBlob struct {
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os.version,omitempty"`
	OSFeatures   []string  `json:"os.features,omitempty"`
	Variant      string    `json:"variant,omitempty"`
	Config       ConfigDetails `json:"config"`
	RootFS       RootFS    `json:"rootfs"`
	History      []History `json:"history"`
	Created      time.Time `json:"created"`
	Author       string    `json:"author,omitempty"`
}

// ConfigDetails contains the runtime configuration
type ConfigDetails struct {
	User         string            `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string          `json:"Env,omitempty"`
	Entrypoint   []string          `json:"Entrypoint,omitempty"`
	Cmd          []string          `json:"Cmd,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string            `json:"WorkingDir,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
}

// RootFS contains information about the root filesystem
type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

// History contains information about image layer history
type History struct {
	Created    time.Time `json:"created"`
	CreatedBy  string    `json:"created_by,omitempty"`
	Author     string    `json:"author,omitempty"`
	Comment    string    `json:"comment,omitempty"`
	EmptyLayer bool      `json:"empty_layer,omitempty"`
}

// parseV2Manifest parses a Docker v2 or OCI manifest
func parseV2Manifest(data []byte) (Manifest, error) {
	var manifest V2Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal v2 manifest")
	}

	// Validate required fields
	if manifest.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported schema version: %d", manifest.SchemaVersion)
	}

	if manifest.Config.Digest == "" {
		return nil, errors.New("manifest missing config digest")
	}

	return &manifest, nil
}

// parseManifestList parses a Docker manifest list or OCI image index
func parseManifestList(data []byte) (Manifest, error) {
	var manifestList ManifestList
	if err := json.Unmarshal(data, &manifestList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal manifest list")
	}

	// Validate required fields
	if manifestList.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported schema version: %d", manifestList.SchemaVersion)
	}

	if len(manifestList.Manifests) == 0 {
		return nil, errors.New("manifest list is empty")
	}

	return &manifestList, nil
}

// parseV1Manifest parses a Docker v1 manifest (legacy)
func parseV1Manifest(data []byte) (Manifest, error) {
	var manifest V1Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal v1 manifest")
	}

	// Validate required fields
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema version: %d", manifest.SchemaVersion)
	}

	return &manifest, nil
}

// ParseImageConfig parses an image configuration blob
func ParseImageConfig(data []byte) (*ImageConfigBlob, error) {
	var config ImageConfigBlob
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal image config")
	}

	return &config, nil
}

// ConvertToPlatform converts various platform representations to our Platform type
func ConvertToPlatform(arch, os, variant, osVersion string, osFeatures []string) Platform {
	return Platform{
		Architecture: arch,
		OS:          os,
		Variant:     variant,
		OSVersion:   osVersion,
		OSFeatures:  osFeatures,
	}
}

// ConvertToImageConfig converts configuration details to our ImageConfig type
func ConvertToImageConfig(config ConfigDetails) ImageConfig {
	return ImageConfig{
		User:         config.User,
		ExposedPorts: config.ExposedPorts,
		Env:          config.Env,
		Entrypoint:   config.Entrypoint,
		Cmd:          config.Cmd,
		Volumes:      config.Volumes,
		WorkingDir:   config.WorkingDir,
	}
}

// ConvertToLayerInfo converts descriptors to layer info
func ConvertToLayerInfo(layers []Descriptor) []LayerInfo {
	layerInfos := make([]LayerInfo, len(layers))
	for i, layer := range layers {
		layerInfos[i] = LayerInfo{
			Digest:    layer.Digest,
			Size:      layer.Size,
			MediaType: layer.MediaType,
		}
	}
	return layerInfos
}

// GetDefaultPlatform returns the default platform for image selection
func GetDefaultPlatform() Platform {
	return Platform{
		Architecture: "amd64",
		OS:           "linux",
	}
}

// NormalizePlatform normalizes platform information
func NormalizePlatform(platform Platform) Platform {
	// Set defaults
	if platform.Architecture == "" {
		platform.Architecture = "amd64"
	}
	if platform.OS == "" {
		platform.OS = "linux"
	}

	// Normalize architecture names
	switch platform.Architecture {
	case "x86_64":
		platform.Architecture = "amd64"
	case "aarch64":
		platform.Architecture = "arm64"
	}

	return platform
}

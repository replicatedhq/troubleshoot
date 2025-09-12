package images

import (
	"encoding/json"
	"testing"
)

func TestParseV2Manifest(t *testing.T) {
	validManifest := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size": 1469,
			"digest": "sha256:config123"
		},
		"layers": [
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size": 977,
				"digest": "sha256:layer123"
			}
		]
	}`

	tests := []struct {
		name     string
		data     []byte
		wantErr  bool
		wantType string
	}{
		{
			name:     "valid v2 manifest",
			data:     []byte(validManifest),
			wantErr:  false,
			wantType: DockerManifestSchema2,
		},
		{
			name:    "invalid json",
			data:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:    "wrong schema version",
			data:    []byte(`{"schemaVersion": 1}`),
			wantErr: true,
		},
		{
			name:    "missing config",
			data:    []byte(`{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.v2+json"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := parseV2Manifest(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseV2Manifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if manifest.GetMediaType() != tt.wantType {
					t.Errorf("parseV2Manifest() mediaType = %v, want %v", manifest.GetMediaType(), tt.wantType)
				}
				if manifest.GetSchemaVersion() != 2 {
					t.Errorf("parseV2Manifest() schemaVersion = %v, want 2", manifest.GetSchemaVersion())
				}
			}
		})
	}
}

func TestParseManifestList(t *testing.T) {
	validManifestList := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
		"manifests": [
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": 1234,
				"digest": "sha256:amd64manifest",
				"platform": {
					"architecture": "amd64",
					"os": "linux"
				}
			},
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": 1235,
				"digest": "sha256:arm64manifest",
				"platform": {
					"architecture": "arm64",
					"os": "linux"
				}
			}
		]
	}`

	tests := []struct {
		name          string
		data          []byte
		wantErr       bool
		wantManifests int
	}{
		{
			name:          "valid manifest list",
			data:          []byte(validManifestList),
			wantErr:       false,
			wantManifests: 2,
		},
		{
			name:    "invalid json",
			data:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:          "empty manifest list",
			data:          []byte(`{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json", "manifests": []}`),
			wantErr:       true,
			wantManifests: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := parseManifestList(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseManifestList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				manifestList := manifest.(*ManifestList)
				if len(manifestList.Manifests) != tt.wantManifests {
					t.Errorf("parseManifestList() manifests count = %v, want %v",
						len(manifestList.Manifests), tt.wantManifests)
				}
			}
		})
	}
}

func TestManifestList_GetManifestForPlatform(t *testing.T) {
	manifestList := &ManifestList{
		SchemaVersion: 2,
		Manifests: []ManifestDescriptor{
			{
				Descriptor: Descriptor{
					Digest: "sha256:amd64manifest",
					Size:   1234,
				},
				Platform: &Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
			{
				Descriptor: Descriptor{
					Digest: "sha256:arm64manifest",
					Size:   1235,
				},
				Platform: &Platform{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
		},
	}

	tests := []struct {
		name           string
		targetPlatform Platform
		wantDigest     string
		wantErr        bool
	}{
		{
			name: "exact match amd64",
			targetPlatform: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
			wantDigest: "sha256:amd64manifest",
			wantErr:    false,
		},
		{
			name: "exact match arm64",
			targetPlatform: Platform{
				Architecture: "arm64",
				OS:           "linux",
			},
			wantDigest: "sha256:arm64manifest",
			wantErr:    false,
		},
		{
			name: "fallback to amd64",
			targetPlatform: Platform{
				Architecture: "unknown",
				OS:           "linux",
			},
			wantDigest: "sha256:amd64manifest",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := manifestList.GetManifestForPlatform(tt.targetPlatform)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetManifestForPlatform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && manifest.Digest != tt.wantDigest {
				t.Errorf("GetManifestForPlatform() digest = %v, want %v", manifest.Digest, tt.wantDigest)
			}
		})
	}
}

func TestParseImageConfig(t *testing.T) {
	validConfig := `{
		"architecture": "amd64",
		"os": "linux",
		"config": {
			"User": "nginx",
			"Env": ["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],
			"Cmd": ["nginx", "-g", "daemon off;"],
			"WorkingDir": "/etc/nginx",
			"Labels": {
				"maintainer": "NGINX Docker Maintainers"
			}
		},
		"created": "2021-01-01T00:00:00Z",
		"rootfs": {
			"type": "layers",
			"diff_ids": ["sha256:layer1", "sha256:layer2"]
		}
	}`

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid config",
			data:    []byte(validConfig),
			wantErr: false,
		},
		{
			name:    "invalid json",
			data:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:    "empty config",
			data:    []byte(`{}`),
			wantErr: false, // Empty config should be allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseImageConfig(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if config == nil {
					t.Error("ParseImageConfig() returned nil config")
				}

				// For valid config, verify some fields
				if string(tt.data) == validConfig {
					if config.Architecture != "amd64" {
						t.Errorf("ParseImageConfig() architecture = %v, want amd64", config.Architecture)
					}
					if config.OS != "linux" {
						t.Errorf("ParseImageConfig() os = %v, want linux", config.OS)
					}
				}
			}
		})
	}
}

func TestConvertToPlatform(t *testing.T) {
	tests := []struct {
		name       string
		arch       string
		os         string
		variant    string
		osVersion  string
		osFeatures []string
		want       Platform
	}{
		{
			name: "basic linux amd64",
			arch: "amd64",
			os:   "linux",
			want: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		},
		{
			name:      "windows with version",
			arch:      "amd64",
			os:        "windows",
			osVersion: "10.0.17763.1234",
			want: Platform{
				Architecture: "amd64",
				OS:           "windows",
				OSVersion:    "10.0.17763.1234",
			},
		},
		{
			name:       "arm with variant",
			arch:       "arm",
			os:         "linux",
			variant:    "v7",
			osFeatures: []string{"feature1"},
			want: Platform{
				Architecture: "arm",
				OS:           "linux",
				Variant:      "v7",
				OSFeatures:   []string{"feature1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToPlatform(tt.arch, tt.os, tt.variant, tt.osVersion, tt.osFeatures)

			if got.Architecture != tt.want.Architecture {
				t.Errorf("ConvertToPlatform() architecture = %v, want %v", got.Architecture, tt.want.Architecture)
			}
			if got.OS != tt.want.OS {
				t.Errorf("ConvertToPlatform() os = %v, want %v", got.OS, tt.want.OS)
			}
			if got.Variant != tt.want.Variant {
				t.Errorf("ConvertToPlatform() variant = %v, want %v", got.Variant, tt.want.Variant)
			}
			if got.OSVersion != tt.want.OSVersion {
				t.Errorf("ConvertToPlatform() osVersion = %v, want %v", got.OSVersion, tt.want.OSVersion)
			}
		})
	}
}

func TestConvertToImageConfig(t *testing.T) {
	configDetails := ConfigDetails{
		User:         "nginx",
		Env:          []string{"PATH=/usr/local/bin"},
		Entrypoint:   []string{"/entrypoint.sh"},
		Cmd:          []string{"nginx", "-g", "daemon off;"},
		WorkingDir:   "/etc/nginx",
		ExposedPorts: map[string]struct{}{"80/tcp": {}},
		Volumes:      map[string]struct{}{"/var/log": {}},
		Labels:       map[string]string{"version": "1.0"},
	}

	imageConfig := ConvertToImageConfig(configDetails)

	if imageConfig.User != configDetails.User {
		t.Errorf("ConvertToImageConfig() user = %v, want %v", imageConfig.User, configDetails.User)
	}

	if len(imageConfig.Env) != len(configDetails.Env) {
		t.Errorf("ConvertToImageConfig() env length = %v, want %v", len(imageConfig.Env), len(configDetails.Env))
	}

	if imageConfig.WorkingDir != configDetails.WorkingDir {
		t.Errorf("ConvertToImageConfig() workingDir = %v, want %v", imageConfig.WorkingDir, configDetails.WorkingDir)
	}
}

func TestConvertToLayerInfo(t *testing.T) {
	descriptors := []Descriptor{
		{
			MediaType: DockerImageLayerTarGzip,
			Size:      1000,
			Digest:    "sha256:layer1",
		},
		{
			MediaType: DockerImageLayerTarGzip,
			Size:      2000,
			Digest:    "sha256:layer2",
		},
	}

	layerInfos := ConvertToLayerInfo(descriptors)

	if len(layerInfos) != len(descriptors) {
		t.Errorf("ConvertToLayerInfo() length = %v, want %v", len(layerInfos), len(descriptors))
	}

	for i, layer := range layerInfos {
		if layer.Digest != descriptors[i].Digest {
			t.Errorf("ConvertToLayerInfo()[%d] digest = %v, want %v", i, layer.Digest, descriptors[i].Digest)
		}
		if layer.Size != descriptors[i].Size {
			t.Errorf("ConvertToLayerInfo()[%d] size = %v, want %v", i, layer.Size, descriptors[i].Size)
		}
		if layer.MediaType != descriptors[i].MediaType {
			t.Errorf("ConvertToLayerInfo()[%d] mediaType = %v, want %v", i, layer.MediaType, descriptors[i].MediaType)
		}
	}
}

func TestGetDefaultPlatform(t *testing.T) {
	platform := GetDefaultPlatform()

	if platform.Architecture != "amd64" {
		t.Errorf("GetDefaultPlatform() architecture = %v, want amd64", platform.Architecture)
	}

	if platform.OS != "linux" {
		t.Errorf("GetDefaultPlatform() os = %v, want linux", platform.OS)
	}
}

func TestNormalizePlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     Platform
	}{
		{
			name:     "empty platform",
			platform: Platform{},
			want: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		},
		{
			name: "x86_64 to amd64",
			platform: Platform{
				Architecture: "x86_64",
				OS:           "linux",
			},
			want: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		},
		{
			name: "aarch64 to arm64",
			platform: Platform{
				Architecture: "aarch64",
				OS:           "linux",
			},
			want: Platform{
				Architecture: "arm64",
				OS:           "linux",
			},
		},
		{
			name: "already normalized",
			platform: Platform{
				Architecture: "amd64",
				OS:           "linux",
				Variant:      "v8",
			},
			want: Platform{
				Architecture: "amd64",
				OS:           "linux",
				Variant:      "v8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePlatform(tt.platform)

			if got.Architecture != tt.want.Architecture {
				t.Errorf("NormalizePlatform() architecture = %v, want %v", got.Architecture, tt.want.Architecture)
			}
			if got.OS != tt.want.OS {
				t.Errorf("NormalizePlatform() os = %v, want %v", got.OS, tt.want.OS)
			}
			if got.Variant != tt.want.Variant {
				t.Errorf("NormalizePlatform() variant = %v, want %v", got.Variant, tt.want.Variant)
			}
		})
	}
}

func BenchmarkParseV2Manifest(b *testing.B) {
	manifest := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size": 1469,
			"digest": "sha256:config123"
		},
		"layers": [
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size": 977,
				"digest": "sha256:layer123"
			}
		]
	}`

	data := []byte(manifest)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseV2Manifest(data)
		if err != nil {
			b.Fatalf("parseV2Manifest failed: %v", err)
		}
	}
}

func TestV2Manifest_Marshal(t *testing.T) {
	manifest := &V2Manifest{
		SchemaVersion: 2,
		MediaType:     DockerManifestSchema2,
		Config: Descriptor{
			MediaType: DockerImageConfig,
			Size:      1469,
			Digest:    "sha256:config123",
		},
		Layers: []Descriptor{
			{
				MediaType: DockerImageLayerTarGzip,
				Size:      977,
				Digest:    "sha256:layer123",
			},
		},
	}

	data, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("V2Manifest.Marshal() error = %v", err)
	}

	// Verify we can parse it back
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("V2Manifest.Marshal() produced invalid JSON: %v", err)
	}

	// Check required fields
	if parsed["schemaVersion"] != float64(2) {
		t.Errorf("V2Manifest.Marshal() schemaVersion = %v, want 2", parsed["schemaVersion"])
	}
}

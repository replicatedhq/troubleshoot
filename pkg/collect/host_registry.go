package collect

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostRegistryImages struct {
	hostCollector *troubleshootv1beta2.HostRegistryImages
	BundlePath    string
}

func (c *CollectHostRegistryImages) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Registry Images")
}

func (c *CollectHostRegistryImages) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostRegistryImages) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	registryInfo := RegistryInfo{
		Images: map[string]RegistryImage{},
	}

	auth := c.resolveAuth()
	for _, image := range c.hostCollector.Images {
		imageRef, err := parseImageRef(image)
		if err != nil {
			registryInfo.Images[image] = RegistryImage{Error: err.Error()}
			continue
		}
		exists, err := imageExistsWithAuth(auth, imageRef, image, 10*time.Second)
		if err != nil {
			registryInfo.Images[image] = RegistryImage{Error: err.Error()}
		} else {
			registryInfo.Images[image] = RegistryImage{Exists: exists}
		}
	}

	b, err := json.MarshalIndent(registryInfo, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal registry info")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	name := filepath.Join("host-collectors/registry-images", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return output, nil
}

func (c *CollectHostRegistryImages) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}

// resolveAuth returns auth config from inline credentials or nil for ambient auth.
func (c *CollectHostRegistryImages) resolveAuth() *registryAuthConfig {
	if c.hostCollector.Username != "" {
		return &registryAuthConfig{
			username: c.hostCollector.Username,
			password: c.hostCollector.Password,
		}
	}
	// No credentials: rely on ambient auth (~/.docker/config.json)
	return nil
}

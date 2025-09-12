package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect/images"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectData struct {
	Collector    *troubleshootv1beta2.Data
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectData) Title() string {
	return getCollectorName(c)
}

func (c *CollectData) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectData) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	// Check if this is an image facts collector (special handling)
	if strings.Contains(c.Collector.CollectorName, "image-facts/") || strings.Contains(c.Collector.Name, "image-facts-") {
		return c.collectImageFacts(progressChan)
	}

	// Default behavior for regular data collectors
	bundlePath := filepath.Join(c.Collector.Name, c.Collector.CollectorName)

	output := NewResult()
	output.SaveResult(c.BundlePath, bundlePath, bytes.NewBuffer([]byte(c.Collector.Data)))

	return output, nil
}

func (c *CollectData) collectImageFacts(progressChan chan<- interface{}) (CollectorResult, error) {
	klog.V(2).Infof("Collecting image facts for namespace: %s", c.Namespace)

	output := NewResult()

	// Create image collection options
	options := images.GetDefaultCollectionOptions()
	options.ContinueOnError = true
	options.IncludeConfig = true
	options.IncludeLayers = false // Don't include layers for faster collection
	options.Timeout = 30000000000 // 30 seconds as nanoseconds

	// Create namespace image collector
	namespaceCollector := images.NewNamespaceImageCollector(c.Client, options)

	// Collect image facts for the namespace
	factsBundle, err := namespaceCollector.CollectNamespaceImageFacts(c.Context, c.Namespace)
	if err != nil {
		klog.Warningf("Failed to collect image facts for namespace %s: %v", c.Namespace, err)
		// Create an error file but don't fail completely
		errorMsg := []byte("Image facts collection failed: " + err.Error())
		errorPath := filepath.Join("image-facts", c.Namespace+"-error.txt")
		output.SaveResult(c.BundlePath, errorPath, bytes.NewBuffer(errorMsg))
		return output, nil
	}

	// If no images found, create an info file
	if len(factsBundle.ImageFacts) == 0 {
		infoMsg := []byte("No container images found in namespace " + c.Namespace)
		infoPath := filepath.Join("image-facts", c.Namespace+"-info.txt")
		output.SaveResult(c.BundlePath, infoPath, bytes.NewBuffer(infoMsg))
		return output, nil
	}

	// Serialize the facts bundle to JSON
	factsJSON, err := json.MarshalIndent(factsBundle, "", "  ")
	if err != nil {
		klog.Errorf("Failed to marshal image facts: %v", err)
		errorMsg := []byte("Failed to serialize image facts: " + err.Error())
		errorPath := filepath.Join("image-facts", c.Namespace+"-error.txt")
		output.SaveResult(c.BundlePath, errorPath, bytes.NewBuffer(errorMsg))
		return output, nil
	}

	// Save the facts.json file
	factsPath := filepath.Join("image-facts", c.Namespace+"-facts.json")
	output.SaveResult(c.BundlePath, factsPath, bytes.NewBuffer(factsJSON))

	// Create summary info
	summaryMsg := []byte(fmt.Sprintf(
		"Image Facts Summary for namespace %s:\n"+
			"- Total Images: %d\n"+
			"- Unique Registries: %d\n"+
			"- Collection Errors: %d\n",
		c.Namespace,
		factsBundle.Summary.TotalImages,
		factsBundle.Summary.UniqueRegistries,
		factsBundle.Summary.CollectionErrors,
	))

	summaryPath := filepath.Join("image-facts", c.Namespace+"-summary.txt")
	output.SaveResult(c.BundlePath, summaryPath, bytes.NewBuffer(summaryMsg))

	klog.V(2).Infof("Image facts collection completed for namespace %s: %d images collected",
		c.Namespace, len(factsBundle.ImageFacts))

	return output, nil
}

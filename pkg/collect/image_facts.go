package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect/images"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectImageFacts struct {
	Collector    *troubleshootv1beta2.Data
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectImageFacts) Title() string {
	return getCollectorName(c)
}

func (c *CollectImageFacts) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectImageFacts) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	klog.V(2).Infof("Collecting image facts for namespace: %s", c.Namespace)

	output := NewResult()

	// Create image collection options
	options := images.GetDefaultCollectionOptions()
	options.ContinueOnError = true
	options.IncludeConfig = true
	options.IncludeLayers = false     // Don't include layers for faster collection
	options.Timeout = 30 * 1000000000 // 30 seconds

	// Create namespace image collector
	namespaceCollector := images.NewNamespaceImageCollector(c.Client, options)

	// Collect image facts for the namespace
	factsBundle, err := namespaceCollector.CollectNamespaceImageFacts(c.Context, c.Namespace)
	if err != nil {
		klog.Warningf("Failed to collect image facts for namespace %s: %v", c.Namespace, err)
		// Create an error file but don't fail completely
		errorMsg := fmt.Sprintf("Image facts collection failed: %v", err)
		errorPath := filepath.Join("image-facts", fmt.Sprintf("%s-error.txt", c.Namespace))
		output.SaveResult(c.BundlePath, errorPath, &FakeReader{data: []byte(errorMsg)})
		return output, nil
	}

	// If no images found, create an info file
	if len(factsBundle.ImageFacts) == 0 {
		infoMsg := fmt.Sprintf("No container images found in namespace %s", c.Namespace)
		infoPath := filepath.Join("image-facts", fmt.Sprintf("%s-info.txt", c.Namespace))
		output.SaveResult(c.BundlePath, infoPath, &FakeReader{data: []byte(infoMsg)})
		return output, nil
	}

	// Serialize the facts bundle to JSON
	factsJSON, err := json.MarshalIndent(factsBundle, "", "  ")
	if err != nil {
		klog.Errorf("Failed to marshal image facts: %v", err)
		errorMsg := fmt.Sprintf("Failed to serialize image facts: %v", err)
		errorPath := filepath.Join("image-facts", fmt.Sprintf("%s-error.txt", c.Namespace))
		output.SaveResult(c.BundlePath, errorPath, &FakeReader{data: []byte(errorMsg)})
		return output, nil
	}

	// Save the facts.json file
	factsPath := filepath.Join("image-facts", fmt.Sprintf("%s-facts.json", c.Namespace))
	output.SaveResult(c.BundlePath, factsPath, &FakeReader{data: factsJSON})

	// Create summary info
	summaryMsg := fmt.Sprintf(`Image Facts Summary for namespace %s:
- Total Images: %d
- Unique Registries: %d
- Total Size: %s
- Collection Errors: %d

Generated at: %s`,
		c.Namespace,
		factsBundle.Summary.TotalImages,
		factsBundle.Summary.UniqueRegistries,
		formatImageSize(factsBundle.Summary.TotalSize),
		factsBundle.Summary.CollectionErrors,
		factsBundle.GeneratedAt.Format("2006-01-02 15:04:05"))

	summaryPath := filepath.Join("image-facts", fmt.Sprintf("%s-summary.txt", c.Namespace))
	output.SaveResult(c.BundlePath, summaryPath, &FakeReader{data: []byte(summaryMsg)})

	klog.V(2).Infof("Image facts collection completed for namespace %s: %d images collected",
		c.Namespace, len(factsBundle.ImageFacts))

	return output, nil
}

// FakeReader implements io.Reader for in-memory data
type FakeReader struct {
	data []byte
	pos  int
}

func (f *FakeReader) Read(p []byte) (n int, err error) {
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}

	n = copy(p, f.data[f.pos:])
	f.pos += n

	if f.pos >= len(f.data) {
		err = io.EOF
	}

	return n, err
}

func formatImageSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

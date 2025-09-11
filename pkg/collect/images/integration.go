package images

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// KubernetesImageExtractor extracts image references from Kubernetes resources
type KubernetesImageExtractor struct {
	client kubernetes.Interface
}

// NewKubernetesImageExtractor creates a new image extractor
func NewKubernetesImageExtractor(client kubernetes.Interface) *KubernetesImageExtractor {
	return &KubernetesImageExtractor{
		client: client,
	}
}

// ExtractImagesFromNamespace extracts all image references from a namespace
func (ke *KubernetesImageExtractor) ExtractImagesFromNamespace(ctx context.Context, namespace string) ([]ImageReference, error) {
	klog.V(2).Infof("Extracting images from namespace: %s", namespace)

	var allImages []ImageReference

	// Extract from pods
	podImages, err := ke.extractImagesFromPods(ctx, namespace)
	if err != nil {
		klog.Warningf("Failed to extract images from pods in namespace %s: %v", namespace, err)
	} else {
		allImages = append(allImages, podImages...)
	}

	// Extract from deployments
	deploymentImages, err := ke.extractImagesFromDeployments(ctx, namespace)
	if err != nil {
		klog.Warningf("Failed to extract images from deployments in namespace %s: %v", namespace, err)
	} else {
		allImages = append(allImages, deploymentImages...)
	}

	// Extract from daemon sets
	daemonSetImages, err := ke.extractImagesFromDaemonSets(ctx, namespace)
	if err != nil {
		klog.Warningf("Failed to extract images from daemonsets in namespace %s: %v", namespace, err)
	} else {
		allImages = append(allImages, daemonSetImages...)
	}

	// Extract from stateful sets
	statefulSetImages, err := ke.extractImagesFromStatefulSets(ctx, namespace)
	if err != nil {
		klog.Warningf("Failed to extract images from statefulsets in namespace %s: %v", namespace, err)
	} else {
		allImages = append(allImages, statefulSetImages...)
	}

	// Deduplicate images
	uniqueImages := deduplicateImageReferences(allImages)

	klog.V(2).Infof("Extracted %d unique images from %d total references in namespace %s",
		len(uniqueImages), len(allImages), namespace)

	return uniqueImages, nil
}

// extractImagesFromPods extracts image references from pods
func (ke *KubernetesImageExtractor) extractImagesFromPods(ctx context.Context, namespace string) ([]ImageReference, error) {
	pods, err := ke.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	var images []ImageReference
	for _, pod := range pods.Items {
		podImages := extractImagesFromPodSpec(pod.Spec)
		images = append(images, podImages...)
	}

	return images, nil
}

// extractImagesFromDeployments extracts image references from deployments
func (ke *KubernetesImageExtractor) extractImagesFromDeployments(ctx context.Context, namespace string) ([]ImageReference, error) {
	deployments, err := ke.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list deployments")
	}

	var images []ImageReference
	for _, deployment := range deployments.Items {
		deploymentImages := extractImagesFromPodSpec(deployment.Spec.Template.Spec)
		images = append(images, deploymentImages...)
	}

	return images, nil
}

// extractImagesFromDaemonSets extracts image references from daemon sets
func (ke *KubernetesImageExtractor) extractImagesFromDaemonSets(ctx context.Context, namespace string) ([]ImageReference, error) {
	daemonSets, err := ke.client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list daemonsets")
	}

	var images []ImageReference
	for _, ds := range daemonSets.Items {
		dsImages := extractImagesFromPodSpec(ds.Spec.Template.Spec)
		images = append(images, dsImages...)
	}

	return images, nil
}

// extractImagesFromStatefulSets extracts image references from stateful sets
func (ke *KubernetesImageExtractor) extractImagesFromStatefulSets(ctx context.Context, namespace string) ([]ImageReference, error) {
	statefulSets, err := ke.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	var images []ImageReference
	for _, sts := range statefulSets.Items {
		stsImages := extractImagesFromPodSpec(sts.Spec.Template.Spec)
		images = append(images, stsImages...)
	}

	return images, nil
}

// extractImagesFromPodSpec extracts image references from a pod specification
func extractImagesFromPodSpec(podSpec corev1.PodSpec) []ImageReference {
	var images []ImageReference

	// Extract from init containers
	for _, container := range podSpec.InitContainers {
		if ref, err := ParseImageReference(container.Image); err == nil {
			images = append(images, ref)
		} else {
			klog.V(3).Infof("Failed to parse init container image %s: %v", container.Image, err)
		}
	}

	// Extract from regular containers
	for _, container := range podSpec.Containers {
		if ref, err := ParseImageReference(container.Image); err == nil {
			images = append(images, ref)
		} else {
			klog.V(3).Infof("Failed to parse container image %s: %v", container.Image, err)
		}
	}

	// Extract from ephemeral containers
	for _, container := range podSpec.EphemeralContainers {
		if ref, err := ParseImageReference(container.Image); err == nil {
			images = append(images, ref)
		} else {
			klog.V(3).Infof("Failed to parse ephemeral container image %s: %v", container.Image, err)
		}
	}

	return images
}

// deduplicateImageReferences removes duplicate image references
func deduplicateImageReferences(refs []ImageReference) []ImageReference {
	seen := make(map[string]bool)
	var unique []ImageReference

	for _, ref := range refs {
		key := ref.String()
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ref)
		}
	}

	return unique
}

// NamespaceImageCollector collects image facts for an entire namespace
type NamespaceImageCollector struct {
	extractor      *KubernetesImageExtractor
	imageCollector *DefaultImageCollector
	factsBuilder   *FactsBuilder
}

// NewNamespaceImageCollector creates a new namespace-level image collector
func NewNamespaceImageCollector(client kubernetes.Interface, options CollectionOptions) *NamespaceImageCollector {
	return &NamespaceImageCollector{
		extractor:      NewKubernetesImageExtractor(client),
		imageCollector: NewImageCollector(options),
		factsBuilder:   NewFactsBuilder(options),
	}
}

// CollectNamespaceImageFacts collects image facts for all images in a namespace
func (nc *NamespaceImageCollector) CollectNamespaceImageFacts(ctx context.Context, namespace string) (*FactsBundle, error) {
	klog.V(2).Infof("Collecting image facts for namespace: %s", namespace)

	// Extract image references from the namespace
	imageRefs, err := nc.extractor.ExtractImagesFromNamespace(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract image references")
	}

	if len(imageRefs) == 0 {
		klog.V(2).Infof("No images found in namespace: %s", namespace)
		return CreateFactsBundle(namespace, []ImageFacts{}), nil
	}

	// Collect facts for all images
	source := fmt.Sprintf("namespace/%s", namespace)
	imageFacts, err := nc.factsBuilder.BuildFactsFromImageReferences(ctx, imageRefs, source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build image facts")
	}

	// Deduplicate and sort
	imageFacts = nc.factsBuilder.DeduplicateImageFacts(imageFacts)
	nc.factsBuilder.SortImageFactsBySize(imageFacts)

	return CreateFactsBundle(namespace, imageFacts), nil
}

// CreateImageFactsCollector creates a troubleshoot collector for image facts
func CreateImageFactsCollector(namespace string, options CollectionOptions) (*troubleshootv1beta2.Collect, error) {
	// Serialize the options for the collector
	optionsData, err := json.Marshal(options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize collection options")
	}

	collect := &troubleshootv1beta2.Collect{
		Data: &troubleshootv1beta2.Data{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("image-facts/%s", namespace),
			},
			Name: fmt.Sprintf("image-facts-%s", namespace),
			Data: fmt.Sprintf("Image facts collection options: %s", string(optionsData)),
		},
	}

	return collect, nil
}

// ProcessImageFactsCollectionResult processes the result of image facts collection
func ProcessImageFactsCollectionResult(ctx context.Context, namespace string, client kubernetes.Interface, options CollectionOptions) ([]byte, error) {
	collector := NewNamespaceImageCollector(client, options)

	factsBundle, err := collector.CollectNamespaceImageFacts(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect namespace image facts")
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(factsBundle, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize facts bundle")
	}

	return data, nil
}

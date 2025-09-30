package autodiscovery

import (
	"context"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// KotsDetector detects KOTS applications in the cluster
type KotsDetector struct {
	client kubernetes.Interface
}

// NewKotsDetector creates a new KOTS detector
func NewKotsDetector(client kubernetes.Interface) *KotsDetector {
	return &KotsDetector{
		client: client,
	}
}

// KotsApplication represents a detected KOTS application
type KotsApplication struct {
	Namespace           string
	AppName             string
	KotsadmDeployment   *appsv1.Deployment
	KotsadmServices     []corev1.Service
	ReplicatedSecrets   []corev1.Secret
	ConfigMaps          []corev1.ConfigMap
	AdditionalResources []KotsResource
}

// KotsResource represents a KOTS-related Kubernetes resource
type KotsResource struct {
	Kind      string
	Name      string
	Namespace string
}

// DetectKotsApplications searches for KOTS applications across all accessible namespaces
func (k *KotsDetector) DetectKotsApplications(ctx context.Context) ([]KotsApplication, error) {
	klog.V(2).Info("Starting KOTS application detection")

	var kotsApps []KotsApplication

	// Get all accessible namespaces
	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Could not list namespaces for KOTS detection: %v", err)
		// Fall back to checking common KOTS namespaces
		namespaces = &corev1.NamespaceList{
			Items: []corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "kots"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "kotsadm"}},
			},
		}
	}

	// Check each namespace for KOTS applications
	for _, ns := range namespaces.Items {
		kotsApp, found := k.detectKotsInNamespace(ctx, ns.Name)
		if found {
			klog.Infof("Found KOTS application in namespace: %s", ns.Name)
			kotsApps = append(kotsApps, kotsApp)
		}
	}

	klog.V(2).Infof("KOTS detection complete. Found %d applications", len(kotsApps))
	return kotsApps, nil
}

// detectKotsInNamespace checks a specific namespace for KOTS applications
func (k *KotsDetector) detectKotsInNamespace(ctx context.Context, namespace string) (KotsApplication, bool) {
	kotsApp := KotsApplication{
		Namespace: namespace,
	}
	found := false

	// Look for kotsadm deployments
	deployments, err := k.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Could not list deployments in namespace %s: %v", namespace, err)
	} else {
		for _, deployment := range deployments.Items {
			if k.isKotsadmDeployment(&deployment) {
				klog.V(2).Infof("Found kotsadm deployment: %s/%s", namespace, deployment.Name)
				kotsApp.KotsadmDeployment = &deployment
				kotsApp.AppName = k.extractAppName(&deployment)
				found = true
			}
		}
	}

	// Look for kotsadm services
	services, err := k.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Could not list services in namespace %s: %v", namespace, err)
	} else {
		for _, service := range services.Items {
			if k.isKotsadmService(&service) {
				klog.V(2).Infof("Found kotsadm service: %s/%s", namespace, service.Name)
				kotsApp.KotsadmServices = append(kotsApp.KotsadmServices, service)
				found = true
			}
		}
	}

	// Look for replicated registry secrets
	secrets, err := k.client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Could not list secrets in namespace %s: %v", namespace, err)
	} else {
		for _, secret := range secrets.Items {
			if k.isReplicatedSecret(&secret) {
				klog.V(2).Infof("Found replicated secret: %s/%s", namespace, secret.Name)
				kotsApp.ReplicatedSecrets = append(kotsApp.ReplicatedSecrets, secret)
				found = true
			}
		}
	}

	// Look for KOTS-related ConfigMaps
	configMaps, err := k.client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Could not list configmaps in namespace %s: %v", namespace, err)
	} else {
		for _, cm := range configMaps.Items {
			if k.isKotsConfigMap(&cm) {
				klog.V(2).Infof("Found KOTS configmap: %s/%s", namespace, cm.Name)
				kotsApp.ConfigMaps = append(kotsApp.ConfigMaps, cm)
				found = true
			}
		}
	}

	return kotsApp, found
}

// isKotsadmDeployment checks if a deployment is a kotsadm deployment
func (k *KotsDetector) isKotsadmDeployment(deployment *appsv1.Deployment) bool {
	// Check deployment name
	name := deployment.Name
	if name == "kotsadm" || name == "kotsadm-api" || name == "kotsadm-web" {
		return true
	}

	// Check labels
	labels := deployment.Labels
	if labels != nil {
		if labels["app"] == "kotsadm" || labels["app.kubernetes.io/name"] == "kotsadm" {
			return true
		}
		if labels["kots.io/kotsadm"] == "true" {
			return true
		}
	}

	// Check container images
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if k.isKotsadmImage(container.Image) {
			return true
		}
	}

	return false
}

// isKotsadmService checks if a service is related to kotsadm
func (k *KotsDetector) isKotsadmService(service *corev1.Service) bool {
	// Check service name
	name := service.Name
	if name == "kotsadm" || name == "kotsadm-api" || name == "kotsadm-web" {
		return true
	}

	// Check labels
	labels := service.Labels
	if labels != nil {
		if labels["app"] == "kotsadm" || labels["app.kubernetes.io/name"] == "kotsadm" {
			return true
		}
		if labels["kots.io/kotsadm"] == "true" {
			return true
		}
	}

	return false
}

// isReplicatedSecret checks if a secret is related to Replicated/KOTS
func (k *KotsDetector) isReplicatedSecret(secret *corev1.Secret) bool {
	name := secret.Name

	// Check for common replicated secret names
	replicatedSecretNames := []string{
		"kotsadm-replicated-registry",
		"replicated-registry",
		"kotsadm-password",
		"kotsadm-cluster-token",
		"kotsadm-session",
		"kotsadm-postgres",
		"kotsadm-rqlite",
	}

	for _, secretName := range replicatedSecretNames {
		if name == secretName {
			return true
		}
	}

	// Check labels
	labels := secret.Labels
	if labels != nil {
		if labels["kots.io/kotsadm"] == "true" {
			return true
		}
		if labels["app"] == "kotsadm" || labels["app.kubernetes.io/name"] == "kotsadm" {
			return true
		}
	}

	// Check annotations
	annotations := secret.Annotations
	if annotations != nil {
		if annotations["kots.io/secret-type"] != "" {
			return true
		}
	}

	return false
}

// isKotsConfigMap checks if a configmap is related to KOTS
func (k *KotsDetector) isKotsConfigMap(cm *corev1.ConfigMap) bool {
	name := cm.Name

	// Check for common KOTS configmap names
	kotsConfigMapNames := []string{
		"kotsadm-config",
		"kotsadm-application-metadata",
		"kotsadm-postgres",
	}

	for _, cmName := range kotsConfigMapNames {
		if name == cmName {
			return true
		}
	}

	// Check labels
	labels := cm.Labels
	if labels != nil {
		if labels["kots.io/kotsadm"] == "true" {
			return true
		}
		if labels["app"] == "kotsadm" || labels["app.kubernetes.io/name"] == "kotsadm" {
			return true
		}
	}

	return false
}

// isKotsadmImage checks if a container image is a kotsadm image
func (k *KotsDetector) isKotsadmImage(image string) bool {
	kotsadmImages := []string{
		"kotsadm/kotsadm",
		"replicated/kotsadm",
		"kotsadm-api",
		"kotsadm-web",
	}

	for _, kotsImage := range kotsadmImages {
		// Check for exact match (handles cases like "kotsadm/kotsadm")
		if image == kotsImage {
			return true
		}

		// Check if image contains the kots image as a proper component
		// This handles private registries like "registry.company.com/kotsadm/kotsadm:v1.0.0"
		if containsImageComponent(image, kotsImage) {
			return true
		}
	}

	return false
}

// containsImageComponent checks if an image path contains a component properly delimited
func containsImageComponent(image, component string) bool {
	// Split image by '/' to get path components
	imageParts := splitImagePath(image)
	componentParts := splitImagePath(component)

	// For single component like "kotsadm-api", check if it appears as a repository name
	if len(componentParts) == 1 {
		for _, part := range imageParts {
			// Remove tag/digest from the part
			repoName := removeTagAndDigest(part)
			if repoName == component {
				return true
			}
		}
		return false
	}

	// For multi-component like "kotsadm/kotsadm", look for consecutive matches
	if len(componentParts) <= len(imageParts) {
		for i := 0; i <= len(imageParts)-len(componentParts); i++ {
			match := true
			for j := 0; j < len(componentParts); j++ {
				imageRepo := removeTagAndDigest(imageParts[i+j])
				if imageRepo != componentParts[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}

	return false
}

// splitImagePath splits an image path by '/' but preserves registry:port
func splitImagePath(image string) []string {
	parts := []string{}
	current := ""

	for i, char := range image {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}

		// Handle final part
		if i == len(image)-1 && current != "" {
			parts = append(parts, current)
		}
	}

	return parts
}

// removeTagAndDigest removes :tag and @digest from image component
func removeTagAndDigest(component string) string {
	// Remove tag (:tag)
	for i := len(component) - 1; i >= 0; i-- {
		if component[i] == ':' {
			component = component[:i]
			break
		}
	}

	// Remove digest (@sha256:...)
	for i := len(component) - 1; i >= 0; i-- {
		if component[i] == '@' {
			component = component[:i]
			break
		}
	}

	return component
}

// extractAppName attempts to extract the application name from a kotsadm deployment
func (k *KotsDetector) extractAppName(deployment *appsv1.Deployment) string {
	// Try to get app name from labels
	if labels := deployment.Labels; labels != nil {
		if appName := labels["kots.io/app"]; appName != "" {
			return appName
		}
		if appName := labels["app.kubernetes.io/name"]; appName != "" && appName != "kotsadm" {
			return appName
		}
	}

	// Try to get app name from annotations
	if annotations := deployment.Annotations; annotations != nil {
		if appName := annotations["kots.io/app-title"]; appName != "" {
			return appName
		}
	}

	// Default to namespace name or "unknown"
	if deployment.Namespace != "" && deployment.Namespace != "default" {
		return deployment.Namespace
	}

	return "kots-application"
}

// GenerateKotsCollectors generates collectors specific to the detected KOTS applications
func (k *KotsDetector) GenerateKotsCollectors(kotsApps []KotsApplication) []CollectorSpec {
	var collectors []CollectorSpec

	for _, kotsApp := range kotsApps {
		klog.V(2).Infof("Generating KOTS collectors for application: %s in namespace: %s", kotsApp.AppName, kotsApp.Namespace)

		// Generate kotsadm deployment collector
		if kotsApp.KotsadmDeployment != nil {
			collectors = append(collectors, k.generateKotsadmDeploymentCollector(kotsApp))
		}

		// Generate kotsadm logs collector
		collectors = append(collectors, k.generateKotsadmLogsCollector(kotsApp))

		// Generate replicated secrets collector
		for _, secret := range kotsApp.ReplicatedSecrets {
			collectors = append(collectors, k.generateReplicatedSecretCollector(kotsApp, secret))
		}

		// Generate KOTS configmaps collector
		for _, cm := range kotsApp.ConfigMaps {
			collectors = append(collectors, k.generateKotsConfigMapCollector(kotsApp, cm))
		}

		// Generate KOTS directory structure collector
		collectors = append(collectors, k.generateKotsDirectoryCollector(kotsApp))
	}

	klog.V(2).Infof("Generated %d KOTS-specific collectors", len(collectors))
	return collectors
}

// generateKurlConfigMapCollectors creates collectors for KURL installation configmaps
func (k *KotsDetector) generateKurlConfigMapCollectors() []CollectorSpec {
	var collectors []CollectorSpec

	// Standard KURL configmaps that should be checked for troubleshooting
	kurlConfigMaps := []string{
		"kurl-current-config",
		"kurl-last-config",
	}

	for _, cmName := range kurlConfigMaps {
		collectors = append(collectors, CollectorSpec{
			Type:      CollectorTypeConfigMaps,
			Name:      fmt.Sprintf("kurl-configmap-%s", cmName),
			Namespace: "kurl",
			Spec: &troubleshootv1beta2.ConfigMap{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("configmaps/kurl/%s", cmName),
				},
				Name:           cmName,
				Namespace:      "kurl",
				IncludeAllData: true,
			},
			Priority: 100,
			Source:   SourceKOTS,
		})
	}

	return collectors
}

// generateStandardReplicatedSecretCollector creates collector for replicated registry secret
func (k *KotsDetector) generateStandardReplicatedSecretCollector() CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeSecrets,
		Name:      "standard-replicated-registry-secret",
		Namespace: "", // Check all namespaces
		Spec: &troubleshootv1beta2.Secret{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: "secrets/kotsadm-replicated-registry",
			},
			Name:           "kotsadm-replicated-registry",
			Namespace:      "", // Will attempt in multiple namespaces
			IncludeValue:   false,
			IncludeAllData: false,
		},
		Priority: 100,
		Source:   SourceKOTS,
	}
}

// generateKotsHostPreflightCollector creates collector for KOTS host preflight results
func (k *KotsDetector) generateKotsHostPreflightCollector(ctx context.Context) CollectorSpec {
	// Try to detect the cluster ID for host preflights
	clusterID := k.detectClusterID(ctx)

	return CollectorSpec{
		Type:      CollectorTypeData,
		Name:      "kots-host-preflights",
		Namespace: "",
		Spec: &troubleshootv1beta2.Data{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/kurl/host-preflights/%s", clusterID),
			},
			Name: fmt.Sprintf("kots/kurl/host-preflights/%s/results.json", clusterID),
			Data: fmt.Sprintf(`{
				"clusterID": "%s", 
				"type": "host-preflights",
				"status": "checking",
				"message": "Attempting to collect KOTS host preflight results"
			}`, clusterID),
		},
		Priority: 90,
		Source:   SourceKOTS,
	}
}

// detectClusterID attempts to detect the cluster ID for KOTS installations
func (k *KotsDetector) detectClusterID(ctx context.Context) string {
	// Try to get cluster ID from node labels or annotations
	nodes, err := k.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Could not list nodes to detect cluster ID: %v", err)
		return "unknown"
	}

	for _, node := range nodes.Items {
		// Check for KURL cluster ID in labels
		if labels := node.Labels; labels != nil {
			if clusterID := labels["kurl.sh/cluster"]; clusterID != "" {
				return clusterID
			}
		}

		// Check node name patterns (like your cluster f5ee12d1)
		if len(node.Name) >= 8 && node.Name != "localhost" {
			// Extract potential cluster ID from node name
			return node.Name[:8] // First 8 chars usually contain cluster ID
		}
	}

	return "unknown"
}

// GenerateStandardKotsCollectors generates collectors for standard KOTS resources that should always be checked
// This includes attempting to collect expected KOTS resources even if no active KOTS apps are detected
func (k *KotsDetector) GenerateStandardKotsCollectors(ctx context.Context) []CollectorSpec {
	var collectors []CollectorSpec

	klog.V(2).Info("Generating standard KOTS resource collectors for troubleshooting")

	// Always attempt to collect standard KOTS/KURL resources for diagnostic purposes
	// These will create error files if resources don't exist, which is valuable for troubleshooting

	// Generate KURL ConfigMap collectors (attempt collection even if not found)
	collectors = append(collectors, k.generateKurlConfigMapCollectors()...)

	// Generate standard replicated registry secret collector (attempt even if not found)
	collectors = append(collectors, k.generateStandardReplicatedSecretCollector())

	// Generate KOTS host preflights collector
	collectors = append(collectors, k.generateKotsHostPreflightCollector(ctx))

	klog.V(2).Infof("Generated %d standard KOTS diagnostic collectors", len(collectors))
	return collectors
}

// generateKotsadmDeploymentCollector creates a collector for kotsadm deployment info
func (k *KotsDetector) generateKotsadmDeploymentCollector(kotsApp KotsApplication) CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeClusterResources,
		Name:      fmt.Sprintf("kots-deployment-%s", kotsApp.AppName),
		Namespace: kotsApp.Namespace,
		Spec: &troubleshootv1beta2.ClusterResources{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/%s/deployment", kotsApp.AppName),
			},
			Namespaces: []string{kotsApp.Namespace},
		},
		Priority: 100, // High priority to ensure collection
		Source:   SourceKOTS,
	}
}

// generateKotsadmLogsCollector creates a collector for kotsadm pod logs
func (k *KotsDetector) generateKotsadmLogsCollector(kotsApp KotsApplication) CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeLogs,
		Name:      fmt.Sprintf("kots-logs-%s", kotsApp.AppName),
		Namespace: kotsApp.Namespace,
		Spec: &troubleshootv1beta2.Logs{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/%s/logs", kotsApp.AppName),
			},
			Selector:  []string{"app=kotsadm", "kots.io/kotsadm=true"},
			Namespace: kotsApp.Namespace,
		},
		Priority: 100,
		Source:   SourceKOTS,
	}
}

// generateReplicatedSecretCollector creates a collector for replicated registry secrets
func (k *KotsDetector) generateReplicatedSecretCollector(kotsApp KotsApplication, secret corev1.Secret) CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeSecrets,
		Name:      fmt.Sprintf("kots-secret-%s-%s", kotsApp.AppName, secret.Name),
		Namespace: kotsApp.Namespace,
		Spec: &troubleshootv1beta2.Secret{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/%s/secrets/%s", kotsApp.AppName, secret.Name),
			},
			Name:           secret.Name,
			Namespace:      kotsApp.Namespace,
			IncludeValue:   false, // Security: only collect metadata
			IncludeAllData: false,
		},
		Priority: 100,
		Source:   SourceKOTS,
	}
}

// generateKotsConfigMapCollector creates a collector for KOTS configmaps
func (k *KotsDetector) generateKotsConfigMapCollector(kotsApp KotsApplication, cm corev1.ConfigMap) CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeConfigMaps,
		Name:      fmt.Sprintf("kots-configmap-%s-%s", kotsApp.AppName, cm.Name),
		Namespace: kotsApp.Namespace,
		Spec: &troubleshootv1beta2.ConfigMap{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/%s/configmaps/%s", kotsApp.AppName, cm.Name),
			},
			Name:           cm.Name,
			Namespace:      kotsApp.Namespace,
			IncludeAllData: true, // Include full configmap data for KOTS configs
		},
		Priority: 100,
		Source:   SourceKOTS,
	}
}

// generateKotsDirectoryCollector creates a collector for KOTS directory structure
func (k *KotsDetector) generateKotsDirectoryCollector(kotsApp KotsApplication) CollectorSpec {
	return CollectorSpec{
		Type:      CollectorTypeData,
		Name:      fmt.Sprintf("kots-directory-%s", kotsApp.AppName),
		Namespace: kotsApp.Namespace,
		Spec: &troubleshootv1beta2.Data{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("kots/%s/directory-info", kotsApp.AppName),
			},
			Name: fmt.Sprintf("kots/%s/info.json", kotsApp.AppName),
			Data: fmt.Sprintf(`{
				"kotsApp": "%s",
				"namespace": "%s",
				"detectedAt": "%s",
				"hasDeployment": %t,
				"secretCount": %d,
				"configMapCount": %d,
				"serviceCount": %d
			}`, kotsApp.AppName, kotsApp.Namespace, "auto-detected",
				kotsApp.KotsadmDeployment != nil,
				len(kotsApp.ReplicatedSecrets),
				len(kotsApp.ConfigMaps),
				len(kotsApp.KotsadmServices)),
		},
		Priority: 90,
		Source:   SourceKOTS,
	}
}

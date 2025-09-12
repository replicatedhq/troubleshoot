package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect/autodiscovery"
	"github.com/replicatedhq/troubleshoot/pkg/collect/images"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)
 
// AutoDiscoveryConfig contains configuration for auto-discovery
type AutoDiscoveryConfig struct {
	Enabled                 bool
	IncludeImages           bool
	RBACCheck               bool
	Profile                 string
	ExcludeNamespaces       []string
	IncludeNamespaces       []string
	IncludeSystemNamespaces bool
	Timeout                 time.Duration
}

// DiscoveryProfile defines different levels of auto-discovery
type DiscoveryProfile struct {
	Name          string
	Description   string
	IncludeImages bool
	RBACCheck     bool
	MaxDepth      int
	Timeout       time.Duration
}

// GetAutoDiscoveryConfig extracts auto-discovery configuration from viper
func GetAutoDiscoveryConfig(v *viper.Viper) AutoDiscoveryConfig {
	return AutoDiscoveryConfig{
		Enabled:                 v.GetBool("auto"),
		IncludeImages:           v.GetBool("include-images"),
		RBACCheck:               v.GetBool("rbac-check"),
		Profile:                 v.GetString("discovery-profile"),
		ExcludeNamespaces:       v.GetStringSlice("exclude-namespaces"),
		IncludeNamespaces:       v.GetStringSlice("include-namespaces"),
		IncludeSystemNamespaces: v.GetBool("include-system-namespaces"),
		Timeout:                 30 * time.Second, // Default timeout
	}
}

// GetDiscoveryProfiles returns available discovery profiles
func GetDiscoveryProfiles() map[string]DiscoveryProfile {
	return map[string]DiscoveryProfile{
		"minimal": {
			Name:          "minimal",
			Description:   "Minimal collection: cluster info, basic logs",
			IncludeImages: false,
			RBACCheck:     true,
			MaxDepth:      1,
			Timeout:       15 * time.Second,
		},
		"standard": {
			Name:          "standard",
			Description:   "Standard collection: logs, configs, secrets, events",
			IncludeImages: false,
			RBACCheck:     true,
			MaxDepth:      2,
			Timeout:       30 * time.Second,
		},
		"comprehensive": {
			Name:          "comprehensive",
			Description:   "Comprehensive collection: everything + image metadata",
			IncludeImages: true,
			RBACCheck:     true,
			MaxDepth:      3,
			Timeout:       60 * time.Second,
		},
		"paranoid": {
			Name:          "paranoid",
			Description:   "Paranoid collection: maximum data with extended timeouts",
			IncludeImages: true,
			RBACCheck:     true,
			MaxDepth:      5,
			Timeout:       120 * time.Second,
		},
	}
}

// ApplyAutoDiscovery applies auto-discovery to the support bundle spec
func ApplyAutoDiscovery(ctx context.Context, client kubernetes.Interface, restConfig *rest.Config,
	mainBundle *troubleshootv1beta2.SupportBundle, config AutoDiscoveryConfig, namespace string) error {

	if !config.Enabled {
		return nil // Auto-discovery not enabled
	}

	klog.V(2).Infof("Applying auto-discovery with profile: %s", config.Profile)

	// Get discovery profile
	profiles := GetDiscoveryProfiles()
	profile, exists := profiles[config.Profile]
	if !exists {
		klog.Warningf("Unknown discovery profile '%s', using 'standard'", config.Profile)
		profile = profiles["standard"]
	}

	// Override profile settings with explicit flags
	if config.IncludeImages {
		profile.IncludeImages = true
	}
	if config.Timeout > 0 {
		profile.Timeout = config.Timeout
	}

	// Create auto-discovery options
	discoveryOpts := autodiscovery.DiscoveryOptions{
		IncludeImages: profile.IncludeImages,
		RBACCheck:     config.RBACCheck,
		MaxDepth:      profile.MaxDepth,
		Timeout:       profile.Timeout,
	}

	// Handle namespace filtering
	if namespace != "" {
		discoveryOpts.Namespaces = []string{namespace}
	} else {
		// Use include/exclude patterns if specified
		if len(config.IncludeNamespaces) > 0 || len(config.ExcludeNamespaces) > 0 {
			// Create namespace scanner to resolve include/exclude patterns
			nsScanner := autodiscovery.NewNamespaceScanner(client)
			scanOpts := autodiscovery.ScanOptions{
				IncludePatterns:         config.IncludeNamespaces,
				ExcludePatterns:         config.ExcludeNamespaces,
				IncludeSystemNamespaces: config.IncludeSystemNamespaces,
			}

			targetNamespaces, err := nsScanner.GetTargetNamespaces(ctx, nil, scanOpts)
			if err != nil {
				klog.Warningf("Failed to resolve namespace patterns, using all accessible namespaces: %v", err)
				// Continue with empty namespace list (all namespaces)
			} else {
				discoveryOpts.Namespaces = targetNamespaces
				klog.V(2).Infof("Resolved namespace patterns to %d namespaces: %v", len(targetNamespaces), targetNamespaces)
			}
		}
	}

	// Create autodiscovery instance
	discoverer, err := autodiscovery.NewDiscoverer(restConfig, client)
	if err != nil {
		return errors.Wrap(err, "failed to create auto-discoverer")
	}

	// Check if we have existing YAML collectors (Path 2) or just auto-discovery (Path 1)
	hasYAMLCollectors := len(mainBundle.Spec.Collectors) > 0

	var autoCollectors []autodiscovery.CollectorSpec

	if hasYAMLCollectors {
		// Path 2: Augment existing YAML collectors with foundational collectors
		klog.V(2).Info("Auto-discovery: Augmenting YAML collectors with foundational collectors (Path 2)")

		// Convert existing collectors to autodiscovery format
		yamlCollectors, err := convertToCollectorSpecs(mainBundle.Spec.Collectors)
		if err != nil {
			return errors.Wrap(err, "failed to convert YAML collectors")
		}

		discoveryOpts.AugmentMode = true
		autoCollectors, err = discoverer.AugmentWithFoundational(ctx, yamlCollectors, discoveryOpts)
		if err != nil {
			return errors.Wrap(err, "failed to augment with foundational collectors")
		}
	} else {
		// Path 1: Pure foundational discovery
		klog.V(2).Info("Auto-discovery: Collecting foundational data only (Path 1)")

		discoveryOpts.FoundationalOnly = true
		autoCollectors, err = discoverer.DiscoverFoundational(ctx, discoveryOpts)
		if err != nil {
			return errors.Wrap(err, "failed to discover foundational collectors")
		}
	}

	// Convert auto-discovered collectors back to troubleshoot specs
	troubleshootCollectors, err := convertToTroubleshootCollectors(autoCollectors)
	if err != nil {
		return errors.Wrap(err, "failed to convert auto-discovered collectors")
	}

	// Update the support bundle spec
	if hasYAMLCollectors {
		// Replace existing collectors with augmented set
		mainBundle.Spec.Collectors = troubleshootCollectors
	} else {
		// Set foundational collectors
		mainBundle.Spec.Collectors = troubleshootCollectors
	}

	klog.V(2).Infof("Auto-discovery complete: %d collectors configured", len(troubleshootCollectors))
	return nil
}

// convertToCollectorSpecs converts troubleshootv1beta2.Collect to autodiscovery.CollectorSpec
func convertToCollectorSpecs(collectors []*troubleshootv1beta2.Collect) ([]autodiscovery.CollectorSpec, error) {
	var specs []autodiscovery.CollectorSpec

	for i, collect := range collectors {
		// Determine collector type and extract relevant information
		spec := autodiscovery.CollectorSpec{
			Priority: 100, // High priority for YAML specs
			Source:   autodiscovery.SourceYAML,
		}

		// Map troubleshoot collectors to autodiscovery types
		switch {
		case collect.Logs != nil:
			spec.Type = autodiscovery.CollectorTypeLogs
			spec.Name = fmt.Sprintf("yaml-logs-%d", i)
			spec.Namespace = collect.Logs.Namespace
			spec.Spec = collect.Logs
		case collect.ConfigMap != nil:
			spec.Type = autodiscovery.CollectorTypeConfigMaps
			spec.Name = fmt.Sprintf("yaml-configmap-%d", i)
			spec.Namespace = collect.ConfigMap.Namespace
			spec.Spec = collect.ConfigMap
		case collect.Secret != nil:
			spec.Type = autodiscovery.CollectorTypeSecrets
			spec.Name = fmt.Sprintf("yaml-secret-%d", i)
			spec.Namespace = collect.Secret.Namespace
			spec.Spec = collect.Secret
		case collect.ClusterInfo != nil:
			spec.Type = autodiscovery.CollectorTypeClusterInfo
			spec.Name = fmt.Sprintf("yaml-clusterinfo-%d", i)
			spec.Spec = collect.ClusterInfo
		case collect.ClusterResources != nil:
			spec.Type = autodiscovery.CollectorTypeClusterResources
			spec.Name = fmt.Sprintf("yaml-clusterresources-%d", i)
			spec.Spec = collect.ClusterResources
		default:
			// For other collector types, create a generic spec
			spec.Type = "other"
			spec.Name = fmt.Sprintf("yaml-other-%d", i)
			spec.Spec = collect
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// convertToTroubleshootCollectors converts autodiscovery.CollectorSpec to troubleshootv1beta2.Collect
func convertToTroubleshootCollectors(collectors []autodiscovery.CollectorSpec) ([]*troubleshootv1beta2.Collect, error) {
	var troubleshootCollectors []*troubleshootv1beta2.Collect

	for _, spec := range collectors {
		collect, err := spec.ToTroubleshootCollect()
		if err != nil {
			klog.Warningf("Failed to convert collector spec %s: %v", spec.Name, err)
			continue
		}
		troubleshootCollectors = append(troubleshootCollectors, collect)
	}

	return troubleshootCollectors, nil
}

// ValidateAutoDiscoveryFlags validates auto-discovery flag combinations
func ValidateAutoDiscoveryFlags(v *viper.Viper) error {
	// If include-images is used without auto, it's an error
	if v.GetBool("include-images") && !v.GetBool("auto") {
		return errors.New("--include-images flag requires --auto flag to be enabled")
	}

	// Validate discovery profile
	profile := v.GetString("discovery-profile")
	profiles := GetDiscoveryProfiles()
	if _, exists := profiles[profile]; !exists {
		return fmt.Errorf("unknown discovery profile: %s. Available profiles: minimal, standard, comprehensive, paranoid", profile)
	}

	// Validate namespace patterns
	includeNS := v.GetStringSlice("include-namespaces")
	excludeNS := v.GetStringSlice("exclude-namespaces")

	if len(includeNS) > 0 && len(excludeNS) > 0 {
		klog.Warning("Both include-namespaces and exclude-namespaces specified. Include patterns take precedence")
	}

	return nil
}

// ShouldUseAutoDiscovery determines if auto-discovery should be used
func ShouldUseAutoDiscovery(v *viper.Viper, args []string) bool {
	// Auto-discovery is enabled by the --auto flag
	autoEnabled := v.GetBool("auto")

	if !autoEnabled {
		return false
	}

	// Auto-discovery can be used with or without YAML specs
	return true
}

// GetAutoDiscoveryMode returns the auto-discovery mode based on arguments
func GetAutoDiscoveryMode(args []string, autoEnabled bool) string {
	if !autoEnabled {
		return "disabled"
	}

	if len(args) == 0 {
		return "foundational-only" // Path 1
	}

	return "yaml-augmented" // Path 2
}

// CreateImageCollectionOptions creates image collection options from CLI config
func CreateImageCollectionOptions(config AutoDiscoveryConfig) images.CollectionOptions {
	options := images.GetDefaultCollectionOptions()

	// Configure based on profile and flags
	profiles := GetDiscoveryProfiles()
	if profile, exists := profiles[config.Profile]; exists {
		options.Timeout = profile.Timeout
		options.IncludeConfig = profile.Name == "comprehensive" || profile.Name == "paranoid"
		options.IncludeLayers = profile.Name == "paranoid"
	}

	// Override based on explicit flags
	if config.Timeout > 0 {
		options.Timeout = config.Timeout
	}

	// For auto-discovery, always continue on error to maximize collection
	options.ContinueOnError = true
	options.EnableCache = true

	return options
}

// PrintAutoDiscoveryInfo prints information about auto-discovery configuration
func PrintAutoDiscoveryInfo(config AutoDiscoveryConfig, mode string) {
	if !config.Enabled {
		return
	}

	fmt.Printf("Auto-discovery enabled (mode: %s, profile: %s)\n", mode, config.Profile)

	if config.IncludeImages {
		fmt.Println("  - Container image metadata collection enabled")
	}

	if len(config.IncludeNamespaces) > 0 {
		fmt.Printf("  - Including namespaces: %v\n", config.IncludeNamespaces)
	}

	if len(config.ExcludeNamespaces) > 0 {
		fmt.Printf("  - Excluding namespaces: %v\n", config.ExcludeNamespaces)
	}

	if config.IncludeSystemNamespaces {
		fmt.Println("  - System namespaces included")
	}

	fmt.Printf("  - RBAC checking: %t\n", config.RBACCheck)
}

package generators

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RequirementParser handles parsing requirement specifications from various sources
type RequirementParser struct {
	validator        *RequirementValidator
	categorizer      *RequirementCategorizer
	conflictResolver *ConflictResolver
}

// NewRequirementParser creates a new requirement parser
func NewRequirementParser() *RequirementParser {
	return &RequirementParser{
		validator:        NewRequirementValidator(),
		categorizer:      NewRequirementCategorizer(),
		conflictResolver: NewConflictResolver(),
	}
}

// ParseFromReader parses requirement specification from a reader
func (p *RequirementParser) ParseFromReader(reader io.Reader, format string) (*RequirementSpec, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read requirement specification: %w", err)
	}

	return p.ParseFromBytes(data, format)
}

// ParseFromBytes parses requirement specification from byte data
func (p *RequirementParser) ParseFromBytes(data []byte, format string) (*RequirementSpec, error) {
	var spec RequirementSpec

	switch strings.ToLower(format) {
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse YAML requirement specification: %w", err)
		}
	case "json":
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse JSON requirement specification: %w", err)
		}
	default:
		// Try to auto-detect format
		if detected, err := p.detectFormat(data); err == nil {
			return p.ParseFromBytes(data, detected)
		}
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	// Validate the parsed specification
	if err := p.validator.Validate(&spec); err != nil {
		return nil, fmt.Errorf("requirement specification validation failed: %w", err)
	}

	return &spec, nil
}

// ParseFromFile parses requirement specification from a file
func (p *RequirementParser) ParseFromFile(filename string) (*RequirementSpec, error) {
	format := strings.TrimPrefix(filepath.Ext(filename), ".")
	if format == "" {
		format = "yaml" // Default to YAML
	}

	data, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	spec, err := p.ParseFromBytes(data, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse requirement file %s: %w", filename, err)
	}

	return spec, nil
}

// ParseMultiple parses multiple requirement specifications and merges them
func (p *RequirementParser) ParseMultiple(sources []RequirementSource) (*RequirementSpec, error) {
	var specs []*RequirementSpec

	for _, source := range sources {
		var spec *RequirementSpec
		var err error

		switch source.Type {
		case "file":
			spec, err = p.ParseFromFile(source.Path)
		case "bytes":
			spec, err = p.ParseFromBytes(source.Data, source.Format)
		case "reader":
			spec, err = p.ParseFromReader(source.Reader, source.Format)
		default:
			return nil, fmt.Errorf("unsupported source type: %s", source.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse source %s: %w", source.Name, err)
		}

		specs = append(specs, spec)
	}

	return p.MergeSpecs(specs)
}

// MergeSpecs merges multiple requirement specifications into one
func (p *RequirementParser) MergeSpecs(specs []*RequirementSpec) (*RequirementSpec, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no specifications to merge")
	}

	if len(specs) == 1 {
		return specs[0], nil
	}

	// Start with the first spec as base
	merged := *specs[0]

	// Merge each subsequent spec
	for i := 1; i < len(specs); i++ {
		if err := p.mergeIntoSpec(&merged, specs[i]); err != nil {
			return nil, fmt.Errorf("failed to merge specification %d: %w", i, err)
		}
	}

	// Categorize requirements to detect conflicts
	categorizedReqs, err := p.categorizer.CategorizeSpec(&merged)
	if err != nil {
		return nil, fmt.Errorf("failed to categorize requirements: %w", err)
	}

	// Resolve any conflicts that arose from merging
	conflicts, err := p.conflictResolver.DetectConflicts(categorizedReqs)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	if len(conflicts) > 0 {
		resolvedReqs, unresolvedConflicts, err := p.conflictResolver.ResolveConflicts(categorizedReqs, conflicts)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve conflicts: %w", err)
		}

		// Log unresolved conflicts as warnings
		if len(unresolvedConflicts) > 0 {
			// In a real implementation, we'd log these warnings
			_ = unresolvedConflicts
		}

		// For simplicity, we'll keep the merged spec as-is
		// In practice, you might want to apply the resolved requirements back to the spec
		_ = resolvedReqs
	}

	// Validate the final merged specification
	if err := p.validator.Validate(&merged); err != nil {
		return nil, fmt.Errorf("merged specification validation failed: %w", err)
	}

	return &merged, nil
}

// RequirementSource represents a source for requirement specifications
type RequirementSource struct {
	Type   string    `json:"type"`   // "file", "bytes", "reader"
	Name   string    `json:"name"`   // Human-readable name
	Path   string    `json:"path"`   // For file type
	Data   []byte    `json:"data"`   // For bytes type
	Reader io.Reader `json:"-"`      // For reader type
	Format string    `json:"format"` // "yaml", "json"
}

// detectFormat attempts to detect the format of requirement data
func (p *RequirementParser) detectFormat(data []byte) (string, error) {
	trimmed := strings.TrimSpace(string(data))

	if strings.HasPrefix(trimmed, "{") {
		return "json", nil
	}

	if strings.Contains(trimmed, "apiVersion:") || strings.Contains(trimmed, "kind:") {
		return "yaml", nil
	}

	// Try to parse as JSON first
	var jsonTest interface{}
	if json.Unmarshal(data, &jsonTest) == nil {
		return "json", nil
	}

	// Try to parse as YAML
	var yamlTest interface{}
	if yaml.Unmarshal(data, &yamlTest) == nil {
		return "yaml", nil
	}

	return "", fmt.Errorf("unable to detect format")
}

// mergeIntoSpec merges source spec into target spec
func (p *RequirementParser) mergeIntoSpec(target, source *RequirementSpec) error {
	// Merge metadata
	p.mergeMetadata(&target.Metadata, &source.Metadata)

	// Merge spec details
	return p.mergeSpecDetails(&target.Spec, &source.Spec)
}

// mergeMetadata merges source metadata into target metadata
func (p *RequirementParser) mergeMetadata(target, source *RequirementMetadata) {
	// Take source values if target is empty
	if target.Name == "" && source.Name != "" {
		target.Name = source.Name
	}
	if target.Description == "" && source.Description != "" {
		target.Description = source.Description
	}
	if target.Version == "" && source.Version != "" {
		target.Version = source.Version
	}
	if target.Vendor == "" && source.Vendor != "" {
		target.Vendor = source.Vendor
	}

	// Merge tags (unique)
	target.Tags = mergeStringSlices(target.Tags, source.Tags)

	// Merge labels and annotations
	if target.Labels == nil {
		target.Labels = make(map[string]string)
	}
	for k, v := range source.Labels {
		target.Labels[k] = v
	}

	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	for k, v := range source.Annotations {
		target.Annotations[k] = v
	}
}

// mergeSpecDetails merges source spec details into target spec details
func (p *RequirementParser) mergeSpecDetails(target, source *RequirementSpecDetails) error {
	// Merge Kubernetes requirements
	p.mergeKubernetesRequirements(&target.Kubernetes, &source.Kubernetes)

	// Merge Resource requirements
	p.mergeResourceRequirements(&target.Resources, &source.Resources)

	// Merge Storage requirements
	p.mergeStorageRequirements(&target.Storage, &source.Storage)

	// Merge Network requirements
	p.mergeNetworkRequirements(&target.Network, &source.Network)

	// Merge Security requirements
	p.mergeSecurityRequirements(&target.Security, &source.Security)

	// Merge Custom requirements
	target.Custom = append(target.Custom, source.Custom...)

	// Merge Vendor requirements
	p.mergeVendorRequirements(&target.Vendor, &source.Vendor)

	// Merge Replicated requirements
	p.mergeReplicatedRequirements(&target.Replicated, &source.Replicated)

	return nil
}

// mergeKubernetesRequirements merges Kubernetes requirements
func (p *RequirementParser) mergeKubernetesRequirements(target, source *KubernetesRequirements) {
	// Version requirements - take most restrictive
	if source.MinVersion != "" {
		if target.MinVersion == "" || p.isVersionGreater(source.MinVersion, target.MinVersion) {
			target.MinVersion = source.MinVersion
		}
	}
	if source.MaxVersion != "" {
		if target.MaxVersion == "" || p.isVersionLess(source.MaxVersion, target.MaxVersion) {
			target.MaxVersion = source.MaxVersion
		}
	}

	// Merge features, APIs, distributions
	target.Features = mergeStringSlices(target.Features, source.Features)
	target.APIs = append(target.APIs, source.APIs...)
	target.Distributions = append(target.Distributions, source.Distributions...)

	// Node count - take most restrictive
	if source.NodeCount.Min > target.NodeCount.Min {
		target.NodeCount.Min = source.NodeCount.Min
	}
	if source.NodeCount.Max > 0 && (target.NodeCount.Max == 0 || source.NodeCount.Max < target.NodeCount.Max) {
		target.NodeCount.Max = source.NodeCount.Max
	}
	if source.NodeCount.Recommended > target.NodeCount.Recommended {
		target.NodeCount.Recommended = source.NodeCount.Recommended
	}
	target.NodeCount.NodeTypes = mergeStringSlices(target.NodeCount.NodeTypes, source.NodeCount.NodeTypes)
}

// mergeResourceRequirements merges resource requirements
func (p *RequirementParser) mergeResourceRequirements(target, source *ResourceRequirements) {
	// CPU requirements - take most restrictive
	if source.CPU.MinCores > target.CPU.MinCores {
		target.CPU.MinCores = source.CPU.MinCores
	}
	if source.CPU.MaxUtilization > 0 && source.CPU.MaxUtilization < target.CPU.MaxUtilization {
		target.CPU.MaxUtilization = source.CPU.MaxUtilization
	}
	target.CPU.RequiredFeatures = mergeStringSlices(target.CPU.RequiredFeatures, source.CPU.RequiredFeatures)
	target.CPU.Architecture = mergeStringSlices(target.CPU.Architecture, source.CPU.Architecture)

	// Memory requirements - take most restrictive
	if source.Memory.MinBytes > target.Memory.MinBytes {
		target.Memory.MinBytes = source.Memory.MinBytes
	}
	if source.Memory.MaxUtilization > 0 && source.Memory.MaxUtilization < target.Memory.MaxUtilization {
		target.Memory.MaxUtilization = source.Memory.MaxUtilization
	}
	if !source.Memory.SwapAllowed {
		target.Memory.SwapAllowed = false
	}

	// Node requirements
	if source.Nodes.MinNodes > target.Nodes.MinNodes {
		target.Nodes.MinNodes = source.Nodes.MinNodes
	}
	if source.Nodes.MaxNodes > 0 && (target.Nodes.MaxNodes == 0 || source.Nodes.MaxNodes < target.Nodes.MaxNodes) {
		target.Nodes.MaxNodes = source.Nodes.MaxNodes
	}

	// Merge node selectors
	if target.Nodes.NodeSelectors == nil {
		target.Nodes.NodeSelectors = make(map[string]string)
	}
	for k, v := range source.Nodes.NodeSelectors {
		target.Nodes.NodeSelectors[k] = v
	}

	// Merge taints and labels
	target.Nodes.Taints = append(target.Nodes.Taints, source.Nodes.Taints...)
	target.Nodes.Labels = append(target.Nodes.Labels, source.Nodes.Labels...)
}

// mergeStorageRequirements merges storage requirements
func (p *RequirementParser) mergeStorageRequirements(target, source *StorageRequirements) {
	// Take most restrictive capacity
	if source.MinCapacity > target.MinCapacity {
		target.MinCapacity = source.MinCapacity
	}

	// Merge storage classes and volume types
	target.StorageClasses = append(target.StorageClasses, source.StorageClasses...)
	target.VolumeTypes = append(target.VolumeTypes, source.VolumeTypes...)

	// Performance requirements - take most restrictive
	if source.Performance.MinIOPS > target.Performance.MinIOPS {
		target.Performance.MinIOPS = source.Performance.MinIOPS
	}
	if source.Performance.MinThroughput > target.Performance.MinThroughput {
		target.Performance.MinThroughput = source.Performance.MinThroughput
	}
	if source.Performance.Latency > 0 && source.Performance.Latency < target.Performance.Latency {
		target.Performance.Latency = source.Performance.Latency
	}

	// Backup requirements - take most restrictive
	if source.Backup.Required {
		target.Backup.Required = true
	}
	if source.Backup.Frequency > 0 && source.Backup.Frequency < target.Backup.Frequency {
		target.Backup.Frequency = source.Backup.Frequency
	}
	if source.Backup.Retention > target.Backup.Retention {
		target.Backup.Retention = source.Backup.Retention
	}
	target.Backup.Destinations = mergeStringSlices(target.Backup.Destinations, source.Backup.Destinations)

	if source.Backup.Encryption {
		target.Backup.Encryption = true
	}
	if source.Backup.Compression {
		target.Backup.Compression = true
	}
	if source.Backup.Validation {
		target.Backup.Validation = true
	}
}

// mergeNetworkRequirements merges network requirements
func (p *RequirementParser) mergeNetworkRequirements(target, source *NetworkRequirements) {
	// Merge connectivity requirements
	target.Connectivity = append(target.Connectivity, source.Connectivity...)

	// Bandwidth - take most restrictive
	if source.Bandwidth.MinUpload > target.Bandwidth.MinUpload {
		target.Bandwidth.MinUpload = source.Bandwidth.MinUpload
	}
	if source.Bandwidth.MinDownload > target.Bandwidth.MinDownload {
		target.Bandwidth.MinDownload = source.Bandwidth.MinDownload
	}
	if source.Bandwidth.Burst > target.Bandwidth.Burst {
		target.Bandwidth.Burst = source.Bandwidth.Burst
	}

	// Latency - take most restrictive
	if source.Latency.MaxRTT > 0 && source.Latency.MaxRTT < target.Latency.MaxRTT {
		target.Latency.MaxRTT = source.Latency.MaxRTT
	}
	if source.Latency.MaxJitter > 0 && source.Latency.MaxJitter < target.Latency.MaxJitter {
		target.Latency.MaxJitter = source.Latency.MaxJitter
	}
	target.Latency.TestEndpoints = mergeStringSlices(target.Latency.TestEndpoints, source.Latency.TestEndpoints)

	// Security requirements - take most restrictive
	if source.Security.TLSRequired {
		target.Security.TLSRequired = true
	}
	if source.Security.MinTLSVersion != "" {
		if target.Security.MinTLSVersion == "" || p.isTLSVersionGreater(source.Security.MinTLSVersion, target.Security.MinTLSVersion) {
			target.Security.MinTLSVersion = source.Security.MinTLSVersion
		}
	}
	target.Security.AllowedCiphers = mergeStringSlices(target.Security.AllowedCiphers, source.Security.AllowedCiphers)

	if source.Security.CertificateAuth {
		target.Security.CertificateAuth = true
	}
	if source.Security.NetworkPolicies {
		target.Security.NetworkPolicies = true
	}

	// DNS requirements
	if source.DNS.Required {
		target.DNS.Required = true
	}
	target.DNS.Servers = mergeStringSlices(target.DNS.Servers, source.DNS.Servers)
	target.DNS.SearchDomains = mergeStringSlices(target.DNS.SearchDomains, source.DNS.SearchDomains)
	if source.DNS.Resolution > 0 && source.DNS.Resolution < target.DNS.Resolution {
		target.DNS.Resolution = source.DNS.Resolution
	}

	// Proxy requirements
	if source.Proxy.Required {
		target.Proxy.Required = true
	}
	if source.Proxy.HTTP != "" {
		target.Proxy.HTTP = source.Proxy.HTTP
	}
	if source.Proxy.HTTPS != "" {
		target.Proxy.HTTPS = source.Proxy.HTTPS
	}
	target.Proxy.NoProxy = mergeStringSlices(target.Proxy.NoProxy, source.Proxy.NoProxy)
	if source.Proxy.Auth {
		target.Proxy.Auth = true
	}
}

// mergeSecurityRequirements merges security requirements
func (p *RequirementParser) mergeSecurityRequirements(target, source *SecurityRequirements) {
	// RBAC requirements
	if source.RBAC.Required {
		target.RBAC.Required = true
	}
	target.RBAC.Roles = append(target.RBAC.Roles, source.RBAC.Roles...)
	if source.RBAC.ClusterRole {
		target.RBAC.ClusterRole = true
	}
	if source.RBAC.ServiceAccount {
		target.RBAC.ServiceAccount = true
	}

	// Pod security requirements
	target.PodSecurity.Standards = mergeStringSlices(target.PodSecurity.Standards, source.PodSecurity.Standards)
	if source.PodSecurity.RunAsNonRoot {
		target.PodSecurity.RunAsNonRoot = true
	}
	if source.PodSecurity.ReadOnlyRoot {
		target.PodSecurity.ReadOnlyRoot = true
	}
	if !source.PodSecurity.AllowPrivileged {
		target.PodSecurity.AllowPrivileged = false
	}
	target.PodSecurity.Capabilities = mergeStringSlices(target.PodSecurity.Capabilities, source.PodSecurity.Capabilities)

	// Enable security features if source requires them
	if source.PodSecurity.SELinux {
		target.PodSecurity.SELinux = true
	}
	if source.PodSecurity.AppArmor {
		target.PodSecurity.AppArmor = true
	}
	if source.PodSecurity.Seccomp {
		target.PodSecurity.Seccomp = true
	}

	// Network policy requirements
	if source.NetworkPolicy.Required {
		target.NetworkPolicy.Required = true
	}
	if source.NetworkPolicy.DefaultDeny {
		target.NetworkPolicy.DefaultDeny = true
	}
	target.NetworkPolicy.IngressPolicies = mergeStringSlices(target.NetworkPolicy.IngressPolicies, source.NetworkPolicy.IngressPolicies)
	target.NetworkPolicy.EgressPolicies = mergeStringSlices(target.NetworkPolicy.EgressPolicies, source.NetworkPolicy.EgressPolicies)
	if source.NetworkPolicy.NamespaceIsolation {
		target.NetworkPolicy.NamespaceIsolation = true
	}

	// Encryption requirements - merge all aspects
	p.mergeEncryptionRequirements(&target.Encryption, &source.Encryption)

	// Admission requirements
	target.Admission.Controllers = append(target.Admission.Controllers, source.Admission.Controllers...)
	target.Admission.Webhooks = append(target.Admission.Webhooks, source.Admission.Webhooks...)
	target.Admission.Policies = append(target.Admission.Policies, source.Admission.Policies...)

	// Compliance requirements
	target.Compliance.Standards = mergeStringSlices(target.Compliance.Standards, source.Compliance.Standards)
	target.Compliance.Frameworks = mergeStringSlices(target.Compliance.Frameworks, source.Compliance.Frameworks)
	target.Compliance.Benchmarks = mergeStringSlices(target.Compliance.Benchmarks, source.Compliance.Benchmarks)

	if source.Compliance.Audit {
		target.Compliance.Audit = true
	}
	if source.Compliance.Reporting {
		target.Compliance.Reporting = true
	}
	if source.Compliance.Remediation {
		target.Compliance.Remediation = true
	}
}

// mergeEncryptionRequirements merges encryption requirements
func (p *RequirementParser) mergeEncryptionRequirements(target, source *EncryptionRequirement) {
	// At rest encryption
	if source.AtRest.Required {
		target.AtRest.Required = true
	}
	if source.AtRest.Algorithm != "" {
		target.AtRest.Algorithm = source.AtRest.Algorithm
	}
	if source.AtRest.KeySize > target.AtRest.KeySize {
		target.AtRest.KeySize = source.AtRest.KeySize
	}
	target.AtRest.Providers = mergeStringSlices(target.AtRest.Providers, source.AtRest.Providers)

	// In transit encryption
	if source.InTransit.Required {
		target.InTransit.Required = true
	}
	if source.InTransit.MinTLSVersion != "" {
		if target.InTransit.MinTLSVersion == "" || p.isTLSVersionGreater(source.InTransit.MinTLSVersion, target.InTransit.MinTLSVersion) {
			target.InTransit.MinTLSVersion = source.InTransit.MinTLSVersion
		}
	}
	target.InTransit.Protocols = mergeStringSlices(target.InTransit.Protocols, source.InTransit.Protocols)
	target.InTransit.CipherSuites = mergeStringSlices(target.InTransit.CipherSuites, source.InTransit.CipherSuites)

	// Key management
	if source.KeyManagement.Provider != "" {
		target.KeyManagement.Provider = source.KeyManagement.Provider
	}
	if source.KeyManagement.Rotation > 0 && source.KeyManagement.Rotation < target.KeyManagement.Rotation {
		target.KeyManagement.Rotation = source.KeyManagement.Rotation
	}
	if source.KeyManagement.Backup {
		target.KeyManagement.Backup = true
	}
	if source.KeyManagement.HSM {
		target.KeyManagement.HSM = true
	}
	if source.KeyManagement.Escrow {
		target.KeyManagement.Escrow = true
	}
}

// mergeVendorRequirements merges vendor-specific requirements
func (p *RequirementParser) mergeVendorRequirements(target, source *VendorRequirements) {
	// This is a simplified merge - in practice, each vendor section would need detailed merging
	// For now, we'll do a basic merge where source overwrites target if not empty

	// AWS requirements
	if !isAWSRequirementsEmpty(&source.AWS) {
		target.AWS = source.AWS
	}

	// Azure requirements
	if !isAzureRequirementsEmpty(&source.Azure) {
		target.Azure = source.Azure
	}

	// GCP requirements
	if !isGCPRequirementsEmpty(&source.GCP) {
		target.GCP = source.GCP
	}

	// VMware requirements
	if !isVMwareRequirementsEmpty(&source.VMware) {
		target.VMware = source.VMware
	}

	// OpenShift requirements
	if !isOpenShiftRequirementsEmpty(&source.OpenShift) {
		target.OpenShift = source.OpenShift
	}

	// Rancher requirements
	if !isRancherRequirementsEmpty(&source.Rancher) {
		target.Rancher = source.Rancher
	}
}

// mergeReplicatedRequirements merges Replicated-specific requirements
func (p *RequirementParser) mergeReplicatedRequirements(target, source *ReplicatedRequirements) {
	// KOTS requirements
	if source.KOTS.Version != "" {
		target.KOTS.Version = source.KOTS.Version
	}
	if source.KOTS.MinVersion != "" {
		if target.KOTS.MinVersion == "" || p.isVersionGreater(source.KOTS.MinVersion, target.KOTS.MinVersion) {
			target.KOTS.MinVersion = source.KOTS.MinVersion
		}
	}

	// Merge boolean flags
	if source.KOTS.AdminConsole {
		target.KOTS.AdminConsole = true
	}
	if source.KOTS.Airgap {
		target.KOTS.Airgap = true
	}
	if source.KOTS.Snapshots {
		target.KOTS.Snapshots = true
	}
	if source.KOTS.Identity {
		target.KOTS.Identity = true
	}

	target.KOTS.Applications = mergeStringSlices(target.KOTS.Applications, source.KOTS.Applications)

	// Registry requirements
	if source.Registry.Type != "" {
		target.Registry = source.Registry
	}

	// License requirements
	if source.License.Type != "" {
		target.License = source.License
	}

	// Preflight requirements
	if source.Preflight.Required {
		target.Preflight.Required = true
	}
	target.Preflight.Analyzers = mergeStringSlices(target.Preflight.Analyzers, source.Preflight.Analyzers)
	target.Preflight.Collectors = mergeStringSlices(target.Preflight.Collectors, source.Preflight.Collectors)
	if source.Preflight.HostChecks {
		target.Preflight.HostChecks = true
	}
	if source.Preflight.RemoteChecks {
		target.Preflight.RemoteChecks = true
	}
	if source.Preflight.Strict {
		target.Preflight.Strict = true
	}

	// Support bundle requirements
	if source.SupportBundle.Required {
		target.SupportBundle.Required = true
	}
	target.SupportBundle.Collectors = mergeStringSlices(target.SupportBundle.Collectors, source.SupportBundle.Collectors)
	target.SupportBundle.Analyzers = mergeStringSlices(target.SupportBundle.Analyzers, source.SupportBundle.Analyzers)
	target.SupportBundle.Redactors = mergeStringSlices(target.SupportBundle.Redactors, source.SupportBundle.Redactors)
	if source.SupportBundle.Size > target.SupportBundle.Size {
		target.SupportBundle.Size = source.SupportBundle.Size
	}
	if source.SupportBundle.Retention > target.SupportBundle.Retention {
		target.SupportBundle.Retention = source.SupportBundle.Retention
	}

	// Config requirements
	target.Config.Items = append(target.Config.Items, source.Config.Items...)
	target.Config.Groups = append(target.Config.Groups, source.Config.Groups...)
	if source.Config.Validation {
		target.Config.Validation = true
	}
	if source.Config.Templates {
		target.Config.Templates = true
	}
	if source.Config.Conditional {
		target.Config.Conditional = true
	}

	// Backup requirements
	if source.Backup.Velero.Version != "" {
		target.Backup.Velero = source.Backup.Velero
	}
	if source.Backup.Snapshots {
		target.Backup.Snapshots = true
	}
	if source.Backup.Schedule != "" {
		target.Backup.Schedule = source.Backup.Schedule
	}
	if source.Backup.Retention > target.Backup.Retention {
		target.Backup.Retention = source.Backup.Retention
	}
	target.Backup.Destinations = append(target.Backup.Destinations, source.Backup.Destinations...)
	if source.Backup.Validation {
		target.Backup.Validation = true
	}
}

// Utility functions for checking empty vendor requirements
func isAWSRequirementsEmpty(req *AWSRequirements) bool {
	return req.EKS.Version == "" && req.IAM.ServiceAccount == false && len(req.Regions) == 0
}

func isAzureRequirementsEmpty(req *AzureRequirements) bool {
	return req.AKS.Version == "" && len(req.Regions) == 0
}

func isGCPRequirementsEmpty(req *GCPRequirements) bool {
	return req.GKE.Version == "" && len(req.Regions) == 0
}

func isVMwareRequirementsEmpty(req *VMwareRequirements) bool {
	return req.VSphere.Version == "" && req.VSAN.Version == ""
}

func isOpenShiftRequirementsEmpty(req *OpenShiftRequirements) bool {
	return req.Version == "" && req.Edition == ""
}

func isRancherRequirementsEmpty(req *RancherRequirements) bool {
	return req.Version == "" && req.Edition == ""
}

// mergeStringSlices merges two string slices, removing duplicates
func mergeStringSlices(target, source []string) []string {
	seen := make(map[string]bool)

	// Add all target items
	for _, item := range target {
		seen[item] = true
	}

	// Add source items if not already present
	for _, item := range source {
		if !seen[item] {
			target = append(target, item)
			seen[item] = true
		}
	}

	return target
}

// Version comparison utilities
func (p *RequirementParser) isVersionGreater(v1, v2 string) bool {
	// Simplified version comparison - in practice would use semantic versioning library
	return strings.Compare(v1, v2) > 0
}

func (p *RequirementParser) isVersionLess(v1, v2 string) bool {
	// Simplified version comparison - in practice would use semantic versioning library
	return strings.Compare(v1, v2) < 0
}

func (p *RequirementParser) isTLSVersionGreater(v1, v2 string) bool {
	// TLS version comparison (1.0 < 1.1 < 1.2 < 1.3)
	versions := map[string]int{
		"1.0": 10,
		"1.1": 11,
		"1.2": 12,
		"1.3": 13,
	}

	val1, ok1 := versions[v1]
	val2, ok2 := versions[v2]

	if !ok1 || !ok2 {
		return false
	}

	return val1 > val2
}

// readFile reads a file from the filesystem (placeholder - would use real file reading)
func readFile(filename string) ([]byte, error) {
	// In a real implementation, this would read from the filesystem
	// For now, return an error to indicate it needs to be implemented
	return nil, fmt.Errorf("file reading not implemented in this example")
}

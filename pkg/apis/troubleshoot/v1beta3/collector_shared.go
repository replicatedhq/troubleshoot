package v1beta3

import (
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

// CollectorMeta contains metadata for collectors
type CollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// Database represents database collectors (PostgreSQL, MySQL, Redis, MSSQL)
// In v1beta3, URI and TLS fields support valueFrom references
type Database struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	// URI can be a literal value or reference to a Secret/ConfigMap
	URI StringOrValueFrom `json:"uri" yaml:"uri"`
	// Parameters for the database connection
	Parameters []string `json:"parameters,omitempty"`
	// TLS configuration with support for valueFrom references
	TLS *TLSParams `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// TLSParams contains TLS configuration
// In v1beta3, certificate fields support valueFrom references
type TLSParams struct {
	// SkipVerify disables TLS verification
	SkipVerify bool `json:"skipVerify,omitempty" yaml:"skipVerify,omitempty"`
	// Secret references a Kubernetes Secret containing TLS materials (v1beta2 compatibility)
	Secret *TLSSecret `json:"secret,omitempty" yaml:"secret,omitempty"`
	// CACert can be a literal value or reference to a Secret/ConfigMap
	CACert StringOrValueFrom `json:"cacert,omitempty" yaml:"cacert,omitempty"`
	// ClientCert can be a literal value or reference to a Secret/ConfigMap
	ClientCert StringOrValueFrom `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	// ClientKey can be a literal value or reference to a Secret/ConfigMap
	ClientKey StringOrValueFrom `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
}

// TLSSecret references a Kubernetes Secret containing TLS materials
// Maintained for backward compatibility
type TLSSecret struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

// Temporary placeholder types for minimal v1beta3 implementation
// These will be properly defined as we expand v1beta3 support
type AfterCollection struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	// TODO: Add fields as needed
}

type Analyze struct {
	// TODO: Add fields as needed
}

type HostAnalyze struct {
	// TODO: Add fields as needed
}

type HostCollect struct {
	// TODO: Add fields as needed
}

// Collect contains all collector definitions
// For phase 1, we're focusing on Database collectors with StringOrValueFrom support
type Collect struct {
	// Database collectors with v1beta3 StringOrValueFrom support
	Postgres *Database `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Mssql    *Database `json:"mssql,omitempty" yaml:"mssql,omitempty"`
	Mysql    *Database `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Redis    *Database `json:"redis,omitempty" yaml:"redis,omitempty"`

	// TODO: Add remaining collector types as we expand v1beta3 support
	// For now, these are placeholders to make the types compile
}

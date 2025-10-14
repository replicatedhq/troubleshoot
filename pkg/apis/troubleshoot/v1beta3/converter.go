package v1beta3

import (
	"context"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
)

// ConvertToV1Beta2WithResolution converts a v1beta3 SupportBundleSpec to v1beta2
// by resolving all StringOrValueFrom fields to their actual values
func ConvertToV1Beta2WithResolution(
	ctx context.Context,
	v3spec *SupportBundleSpec,
	client kubernetes.Interface,
	defaultNamespace string,
) (*troubleshootv1beta2.SupportBundleSpec, error) {
	v2spec := &troubleshootv1beta2.SupportBundleSpec{
		Uri:                    v3spec.Uri,
		RunHostCollectorsInPod: v3spec.RunHostCollectorsInPod,
	}

	// Convert collectors
	if v3spec.Collectors != nil {
		v2collectors := make([]*troubleshootv1beta2.Collect, 0, len(v3spec.Collectors))
		for _, v3collector := range v3spec.Collectors {
			v2collector, err := convertCollector(ctx, v3collector, client, defaultNamespace)
			if err != nil {
				return nil, fmt.Errorf("failed to convert collector: %w", err)
			}
			v2collectors = append(v2collectors, v2collector)
		}
		v2spec.Collectors = v2collectors
	}

	// TODO: Convert AfterCollection, HostCollectors, Analyzers, HostAnalyzers when v1beta3 support is expanded

	return v2spec, nil
}

// convertCollector converts a v1beta3 Collect to v1beta2 Collect
func convertCollector(
	ctx context.Context,
	v3collector *Collect,
	client kubernetes.Interface,
	defaultNamespace string,
) (*troubleshootv1beta2.Collect, error) {
	v2collector := &troubleshootv1beta2.Collect{}

	// Convert database collectors
	if v3collector.Postgres != nil {
		db, err := convertDatabase(ctx, v3collector.Postgres, client, defaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to convert postgres collector: %w", err)
		}
		v2collector.Postgres = db
	}

	if v3collector.Mysql != nil {
		db, err := convertDatabase(ctx, v3collector.Mysql, client, defaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mysql collector: %w", err)
		}
		v2collector.Mysql = db
	}

	if v3collector.Mssql != nil {
		db, err := convertDatabase(ctx, v3collector.Mssql, client, defaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mssql collector: %w", err)
		}
		v2collector.Mssql = db
	}

	if v3collector.Redis != nil {
		db, err := convertDatabase(ctx, v3collector.Redis, client, defaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to convert redis collector: %w", err)
		}
		v2collector.Redis = db
	}

	// TODO: Add conversion for other collector types as v1beta3 support expands

	return v2collector, nil
}

// convertDatabase converts a v1beta3 Database to v1beta2 Database
func convertDatabase(
	ctx context.Context,
	v3db *Database,
	client kubernetes.Interface,
	defaultNamespace string,
) (*troubleshootv1beta2.Database, error) {
	// Resolve URI
	uri, err := ResolveStringOrValueFrom(ctx, v3db.URI, client, defaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database URI: %w", err)
	}

	v2db := &troubleshootv1beta2.Database{
		CollectorMeta: troubleshootv1beta2.CollectorMeta{
			CollectorName: v3db.CollectorName,
			Exclude:       v3db.Exclude,
		},
		URI:        uri,
		Parameters: v3db.Parameters,
	}

	// Convert TLS params if present
	if v3db.TLS != nil {
		tlsParams, err := convertTLSParams(ctx, v3db.TLS, client, defaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to convert TLS params: %w", err)
		}
		v2db.TLS = tlsParams
	}

	return v2db, nil
}

// convertTLSParams converts v1beta3 TLSParams to v1beta2 TLSParams
func convertTLSParams(
	ctx context.Context,
	v3tls *TLSParams,
	client kubernetes.Interface,
	defaultNamespace string,
) (*troubleshootv1beta2.TLSParams, error) {
	v2tls := &troubleshootv1beta2.TLSParams{
		SkipVerify: v3tls.SkipVerify,
	}

	// Preserve v1beta2 Secret reference if present (backward compatibility)
	if v3tls.Secret != nil {
		v2tls.Secret = &troubleshootv1beta2.TLSSecret{
			Name:      v3tls.Secret.Name,
			Namespace: v3tls.Secret.Namespace,
		}
	}

	// Resolve v1beta3 StringOrValueFrom fields
	caCert, err := ResolveStringOrValueFrom(ctx, v3tls.CACert, client, defaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve CA cert: %w", err)
	}
	v2tls.CACert = caCert

	clientCert, err := ResolveStringOrValueFrom(ctx, v3tls.ClientCert, client, defaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve client cert: %w", err)
	}
	v2tls.ClientCert = clientCert

	clientKey, err := ResolveStringOrValueFrom(ctx, v3tls.ClientKey, client, defaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve client key: %w", err)
	}
	v2tls.ClientKey = clientKey

	return v2tls, nil
}

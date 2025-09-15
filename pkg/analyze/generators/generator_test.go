package generators

import (
	"context"
	"testing"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyzerGenerator(t *testing.T) {
	gen := NewAnalyzerGenerator()

	assert.NotNil(t, gen)
	assert.NotNil(t, gen.templates)
	assert.NotNil(t, gen.validators)

	// Check that default templates and validators are registered
	assert.NotEmpty(t, gen.templates)
	assert.NotEmpty(t, gen.validators)
}

func TestAnalyzerGenerator_GenerateAnalyzers(t *testing.T) {
	gen := NewAnalyzerGenerator()
	ctx := context.Background()

	tests := []struct {
		name         string
		requirements *analyzer.RequirementSpec
		opts         *GenerationOptions
		wantErr      bool
		errMsg       string
		wantSpecs    int
	}{
		{
			name:         "nil requirements",
			requirements: nil,
			opts:         nil,
			wantErr:      true,
			errMsg:       "requirements cannot be nil",
		},
		{
			name: "kubernetes version requirements",
			requirements: &analyzer.RequirementSpec{
				APIVersion: "troubleshoot.replicated.com/v1beta2",
				Kind:       "Requirements",
				Metadata: analyzer.RequirementMetadata{
					Name: "k8s-version-test",
				},
				Spec: analyzer.RequirementSpecDetails{
					Kubernetes: analyzer.KubernetesRequirements{
						MinVersion: "1.20.0",
						MaxVersion: "1.25.0",
					},
				},
			},
			opts:      nil,
			wantErr:   false,
			wantSpecs: 1,
		},
		{
			name: "comprehensive requirements",
			requirements: &analyzer.RequirementSpec{
				APIVersion: "troubleshoot.replicated.com/v1beta2",
				Kind:       "Requirements",
				Metadata: analyzer.RequirementMetadata{
					Name: "comprehensive-test",
				},
				Spec: analyzer.RequirementSpecDetails{
					Kubernetes: analyzer.KubernetesRequirements{
						MinVersion: "1.20.0",
						Required:   []string{"ingress-nginx", "cert-manager"},
					},
					Resources: analyzer.ResourceRequirements{
						CPU: analyzer.ResourceRequirement{
							Min: "4",
						},
						Memory: analyzer.ResourceRequirement{
							Min: "8Gi",
						},
					},
					Storage: analyzer.StorageRequirements{
						Classes:     []string{"fast-ssd"},
						MinCapacity: "100Gi",
						AccessModes: []string{"ReadWriteOnce"},
					},
					Network: analyzer.NetworkRequirements{
						Ports: []analyzer.PortRequirement{
							{Port: 80, Protocol: "TCP", Required: true},
							{Port: 443, Protocol: "TCP", Required: true},
						},
						Connectivity: []string{"https://api.example.com"},
					},
					Custom: []analyzer.CustomRequirement{
						{
							Name:      "database-connection",
							Type:      "database",
							Condition: "available",
							Context: map[string]interface{}{
								"uri": "postgresql://localhost:5432/mydb",
							},
						},
					},
				},
			},
			opts:      &GenerationOptions{IncludeOptional: true},
			wantErr:   false,
			wantSpecs: 8, // k8s(2) + resources(2) + storage(2) + network(2) + custom(1) = 9, but some might be combined
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, err := gen.GenerateAnalyzers(ctx, tt.requirements, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, specs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, specs)
				assert.GreaterOrEqual(t, len(specs), 1)

				// Verify all specs have required fields
				for i, spec := range specs {
					assert.NotEmpty(t, spec.Name, "spec %d should have name", i)
					assert.NotEmpty(t, spec.Type, "spec %d should have type", i)
					assert.NotEmpty(t, spec.Category, "spec %d should have category", i)
					assert.Greater(t, spec.Priority, 0, "spec %d should have positive priority", i)
					assert.NotNil(t, spec.Config, "spec %d should have config", i)
				}

				// Check specs are sorted by priority (higher first)
				for i := 1; i < len(specs); i++ {
					assert.GreaterOrEqual(t, specs[i-1].Priority, specs[i].Priority,
						"specs should be sorted by priority (higher first)")
				}
			}
		})
	}
}

func TestAnalyzerGenerator_generateVersionOutcomes(t *testing.T) {
	gen := NewAnalyzerGenerator()

	tests := []struct {
		name       string
		minVersion string
		maxVersion string
		wantPass   bool
		wantFail   bool
	}{
		{
			name:       "min and max version",
			minVersion: "1.20.0",
			maxVersion: "1.25.0",
			wantPass:   true,
			wantFail:   true,
		},
		{
			name:       "min version only",
			minVersion: "1.20.0",
			maxVersion: "",
			wantPass:   true,
			wantFail:   true,
		},
		{
			name:       "max version only",
			minVersion: "",
			maxVersion: "1.25.0",
			wantPass:   true,
			wantFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcomes := gen.generateVersionOutcomes(tt.minVersion, tt.maxVersion)

			assert.NotEmpty(t, outcomes)

			var hasPass, hasFail bool
			for _, outcome := range outcomes {
				if _, ok := outcome["pass"]; ok {
					hasPass = true
				}
				if _, ok := outcome["fail"]; ok {
					hasFail = true
				}
			}

			if tt.wantPass {
				assert.True(t, hasPass, "should have pass outcome")
			}
			if tt.wantFail {
				assert.True(t, hasFail, "should have fail outcome")
			}
		})
	}
}

func TestAnalyzerGenerator_ValidateRequirements(t *testing.T) {
	gen := NewAnalyzerGenerator()
	ctx := context.Background()

	tests := []struct {
		name         string
		requirements *analyzer.RequirementSpec
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "nil requirements",
			requirements: nil,
			wantErr:      true,
			errMsg:       "requirements cannot be nil",
		},
		{
			name: "valid requirements",
			requirements: &analyzer.RequirementSpec{
				Spec: analyzer.RequirementSpecDetails{
					Kubernetes: analyzer.KubernetesRequirements{
						MinVersion: "v1.20.0",
					},
					Resources: analyzer.ResourceRequirements{
						CPU: analyzer.ResourceRequirement{
							Min: "2",
						},
						Memory: analyzer.ResourceRequirement{
							Min: "4Gi",
						},
					},
					Storage: analyzer.StorageRequirements{
						MinCapacity: "100Gi",
						AccessModes: []string{"ReadWriteOnce"},
					},
					Network: analyzer.NetworkRequirements{
						Ports: []analyzer.PortRequirement{
							{Port: 80, Protocol: "TCP", Required: true},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version format",
			requirements: &analyzer.RequirementSpec{
				Spec: analyzer.RequirementSpecDetails{
					Kubernetes: analyzer.KubernetesRequirements{
						MinVersion: "invalid-version",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid version format",
		},
		{
			name: "invalid port number",
			requirements: &analyzer.RequirementSpec{
				Spec: analyzer.RequirementSpecDetails{
					Network: analyzer.NetworkRequirements{
						Ports: []analyzer.PortRequirement{
							{Port: -1, Protocol: "TCP"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid port number",
		},
		{
			name: "invalid access mode",
			requirements: &analyzer.RequirementSpec{
				Spec: analyzer.RequirementSpecDetails{
					Storage: analyzer.StorageRequirements{
						AccessModes: []string{"InvalidMode"},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid access mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.ValidateRequirements(ctx, tt.requirements)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnalyzerGenerator_RegisterTemplate(t *testing.T) {
	gen := NewAnalyzerGenerator()

	tests := []struct {
		name     string
		tempName string
		template AnalyzerTemplate
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid template",
			tempName: "test-template",
			template: AnalyzerTemplate{
				Name:        "Test Template",
				Description: "Test description",
				Generator:   func(ctx context.Context, req interface{}) ([]analyzer.AnalyzerSpec, error) { return nil, nil },
			},
			wantErr: false,
		},
		{
			name:     "empty template name",
			tempName: "",
			template: AnalyzerTemplate{
				Generator: func(ctx context.Context, req interface{}) ([]analyzer.AnalyzerSpec, error) { return nil, nil },
			},
			wantErr: true,
			errMsg:  "template name cannot be empty",
		},
		{
			name:     "nil generator",
			tempName: "test-template",
			template: AnalyzerTemplate{
				Name:      "Test Template",
				Generator: nil,
			},
			wantErr: true,
			errMsg:  "template generator cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.RegisterTemplate(tt.tempName, tt.template)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnalyzerGenerator_CustomAnalyzers(t *testing.T) {
	gen := NewAnalyzerGenerator()
	ctx := context.Background()

	// Test database analyzer generation
	customReq := &analyzer.CustomRequirement{
		Name: "test-db",
		Type: "database",
		Context: map[string]interface{}{
			"uri": "postgresql://localhost:5432/test",
		},
	}

	specs, err := gen.generateDatabaseAnalyzer(ctx, customReq)
	require.NoError(t, err)
	require.Len(t, specs, 1)

	spec := specs[0]
	assert.Equal(t, "database-test-db", spec.Name)
	assert.Equal(t, "database", spec.Type)
	assert.Equal(t, "database", spec.Category)
	assert.NotNil(t, spec.Config)

	// Test API analyzer generation
	apiReq := &analyzer.CustomRequirement{
		Name: "test-api",
		Type: "api",
		Context: map[string]interface{}{
			"url": "https://api.example.com/health",
		},
	}

	specs, err = gen.generateAPIAnalyzer(ctx, apiReq)
	require.NoError(t, err)
	require.Len(t, specs, 1)

	spec = specs[0]
	assert.Equal(t, "api-test-api", spec.Name)
	assert.Equal(t, "http", spec.Type)
	assert.Equal(t, "api", spec.Category)
}

func TestAnalyzerGenerator_filterByCategory(t *testing.T) {
	gen := NewAnalyzerGenerator()

	specs := []analyzer.AnalyzerSpec{
		{Name: "k8s-1", Category: "kubernetes"},
		{Name: "res-1", Category: "resources"},
		{Name: "k8s-2", Category: "kubernetes"},
		{Name: "net-1", Category: "network"},
	}

	tests := []struct {
		name       string
		categories []string
		wantCount  int
	}{
		{
			name:       "no filter",
			categories: []string{},
			wantCount:  4,
		},
		{
			name:       "single category",
			categories: []string{"kubernetes"},
			wantCount:  2,
		},
		{
			name:       "multiple categories",
			categories: []string{"kubernetes", "network"},
			wantCount:  3,
		},
		{
			name:       "non-existent category",
			categories: []string{"non-existent"},
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := gen.filterByCategory(specs, tt.categories)
			assert.Len(t, filtered, tt.wantCount)
		})
	}
}

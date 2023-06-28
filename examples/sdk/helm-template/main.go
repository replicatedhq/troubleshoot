package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	tsloader "github.com/replicatedhq/troubleshoot/pkg/loader"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"sigs.k8s.io/yaml"
)

func main() {
	// Load chart from tarball
	chart, err := loader.LoadFile("mychart-0.1.0.tgz")
	if err != nil {
		log.Fatalf("error loading at chart: %v", err)
	}

	// Render chart to manifests. Equivalent to `helm template --values values.yaml`
	renderedManifests, err := renderChartToManifests(chart, chartutil.Values{
		"preflight": true,
		"includeTroubleshootCRs": true,
	})
	if err != nil {
		log.Fatalf("error rendering chart: %v", err)
	}

	// Load rendered manifests into a slice of troubleshoot specs
	// tsKinds can now be used to run preflights and collect support bundles
	ctx := context.Background()
	tsKinds, err := tsloader.LoadSpecs(ctx, tsloader.LoadOptions{
		RawSpec: renderedManifests,
	})
	if err != nil {
		log.Fatalf("error loading troubleshoot specs: %v", err)
	}

	// Ensure we have the correct number of specs
	if len(tsKinds.PreflightsV1Beta2) != 1 {
		log.Fatalf("expected 1 preflight, got %d", len(tsKinds.PreflightsV1Beta2))
	}
	if len(tsKinds.SupportBundlesV1Beta2) != 1 {
		log.Fatalf("expected 1 supportbundle, got %d", len(tsKinds.SupportBundlesV1Beta2))
	}
	if len(tsKinds.RedactorsV1Beta2) != 1 {
		log.Fatalf("expected 1 redactor, got %d", len(tsKinds.RedactorsV1Beta2))
	}

	// Print the specs as yaml
	fmt.Println("---\n############# Support bundle #############")
	printSpecAsYaml(tsKinds.SupportBundlesV1Beta2[0])
	fmt.Println("---\n############# Preflight #############")
	printSpecAsYaml(tsKinds.PreflightsV1Beta2[0])
	fmt.Println("---\n############# Redactor #############")
	printSpecAsYaml(tsKinds.RedactorsV1Beta2[0])
}

func renderChartToManifests(chart *chart.Chart, inputValues chartutil.Values) (string, error) {
	options := chartutil.ReleaseOptions{
		Name: "my-release",
	}

	// Pull in imported values from dependencies to the parent chart
	if err := chartutil.ProcessDependencies(chart, chartutil.Values{}); err != nil {
		return "", fmt.Errorf("failed to process chart %q dependencies: %w", chart.Name(), err)
	}

	// Gather all values necessary to render the templates
	rValues, err := chartutil.ToRenderValues(chart, inputValues, options, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render template values: %w", err)
	}

	// Render the chart templates
	renderedTemplates, err := engine.Render(chart, rValues)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// Combine all rendered templates into a single yaml multidoc string
	// Only pick up YAML files, ignore any others
	var out strings.Builder
	for k, v := range renderedTemplates {
		if strings.HasSuffix(k, ".yaml") || strings.HasSuffix(k, ".yml") {
			// Add the multidoc split marker
			out.WriteString(fmt.Sprintf("---\n# Source: %s\n%s\n", k, v))
		}
	}

	return out.String(), nil
}

func printSpecAsYaml(v any) {
	b, err := yaml.Marshal(v)
	if err != nil {
		log.Fatalf("error marshalling troubleshoot specs: %v", err)
	}

	fmt.Println(string(b))
}

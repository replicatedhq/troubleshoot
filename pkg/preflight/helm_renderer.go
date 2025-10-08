package preflight

import (
	"fmt"
	"sort"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// keepHelmImports ensures Helm modules are retained by the linker until we wire them in.
var _ any = func() any {
	_ = engine.Engine{}
	_ = chart.Chart{}
	_ = chartutil.Values{}
	return nil
}()

// RenderWithHelmTemplate renders a single YAML template string using Helm's engine
// with the provided values (corresponding to .Values in Helm templates).
func RenderWithHelmTemplate(templateContent string, values map[string]interface{}) (string, error) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "preflight-templating",
			APIVersion: chart.APIVersionV2,
			Type:       "application",
		},
		Templates: []*chart.File{
			{
				Name: "templates/preflight.yaml",
				Data: []byte(templateContent),
			},
		},
	}

	releaseOpts := chartutil.ReleaseOptions{
		Name:      "preflight",
		Namespace: "default",
		IsInstall: true,
		IsUpgrade: false,
		Revision:  1,
	}
	caps := chartutil.DefaultCapabilities

	renderVals, err := chartutil.ToRenderValues(ch, chartutil.Values(values), releaseOpts, caps)
	if err != nil {
		return "", fmt.Errorf("build render values: %w", err)
	}

	eng := engine.Engine{}
	out, err := eng.Render(ch, renderVals)
	if err != nil {
		return "", fmt.Errorf("helm render: %w", err)
	}
	if len(out) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return out[keys[0]], nil
}

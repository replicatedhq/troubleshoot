package helm

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	helmcli "helm.sh/helm/v3/pkg/cli"
	helmvalues "helm.sh/helm/v3/pkg/cli/values"
	helmengine "helm.sh/helm/v3/pkg/engine"
	helmgetter "helm.sh/helm/v3/pkg/getter"
)

func ApplyTemplates(content []byte, valuesFiles []string, valuesData []byte) ([]byte, error) {
	if len(valuesFiles) == 0 && len(valuesData) == 0 {
		return content, nil
	}

	opts := helmvalues.Options{
		ValueFiles: valuesFiles,
	}

	if len(valuesData) > 0 {
		// There is no option to pass in YAML values.  We can pass in JSON or create a temp file with YAML.
		file, err := ioutil.TempFile("", "values-data-")
		if err != nil {
			return nil, errors.Wrap(err, "create temp values file")
		}
		defer file.Close()
		defer os.RemoveAll(file.Name())

		_, err = file.Write(valuesData)
		if err != nil {
			return nil, errors.Wrap(err, "save values to temp file")
		}

		file.Close()
		opts.ValueFiles = append(opts.ValueFiles, file.Name())
	}

	settings := helmcli.New()
	p := helmgetter.All(settings)

	vals, err := opts.MergeValues(p)
	if err != nil {
		return nil, errors.Wrap(err, "merge values")
	}

	fakechart := &helmchart.Chart{
		Metadata: &helmchart.Metadata{
			Name: "preflight",
		},
		Values: make(map[string]interface{}),
		Templates: []*helmchart.File{
			{
				Name: "preflight.yaml",
				Data: content,
			},
		},
	}

	options := chartutil.ReleaseOptions{
		Name:      "",
		Namespace: "",
		Revision:  1,
		IsInstall: false,
		IsUpgrade: false,
	}
	valuesToRender, err := chartutil.ToRenderValues(fakechart, vals, options, nil)
	if err != nil {
		return nil, errors.Wrap(err, "render values")
	}

	renderedFiles, err := helmengine.Render(fakechart, valuesToRender)
	if err != nil {
		return nil, errors.Wrap(err, "render chart")
	}

	for _, renderedFile := range renderedFiles {
		// only one file is expected
		return []byte(renderedFile), nil
	}

	return nil, errors.New("render did not return any files")
}

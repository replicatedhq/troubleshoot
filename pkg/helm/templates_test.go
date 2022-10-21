package helm

import (
	_ "embed"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/spec-before.yaml
var specBeforeTemplating string

//go:embed testdata/spec-after.yaml
var specAfterTemplating string

//go:embed testdata/values.yaml
var valuesData string

//go:embed testdata/additional-values.yaml
var additionalValuesData string

//go:embed testdata/malformed-template.yaml
var malformedTemplate string

func Test_ApplyTemplates(t *testing.T) {
	tests := []struct {
		name          string
		withTemplates string
		valuesData    string
		want          string
		isError       bool
	}{
		{
			name:          "helm templates",
			withTemplates: specBeforeTemplating,
			valuesData:    valuesData,
			want:          specAfterTemplating,
			isError:       false,
		},
		{
			name:          "malformed template",
			withTemplates: malformedTemplate,
			valuesData:    valuesData,
			want:          "",
			isError:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			valuesFile, err := ioutil.TempFile("", "helm-template-test-")
			req.NoError(err)
			defer os.RemoveAll(valuesFile.Name())

			_, err = io.Copy(valuesFile, strings.NewReader(test.valuesData))
			req.NoError(err)

			valuesFile.Close()
			got, err := ApplyTemplates([]byte(test.withTemplates), []string{valuesFile.Name()}, []byte(additionalValuesData))

			if test.isError {
				req.Error(err)
				return
			}

			req.NoError(err)
			assert.Equal(t, test.want, string(got))
			_, err = ApplyTemplates([]byte(test.withTemplates), []string{valuesFile.Name()}, nil)
			req.Error(err)
		})
	}
}

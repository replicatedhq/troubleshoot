package docrewrite

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func ConvertToV1Beta2(doc []byte) ([]byte, error) {
	var parsed map[string]interface{}
	err := yaml.Unmarshal(doc, &parsed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal yaml")
	}

	v, ok := parsed["apiVersion"]
	if !ok {
		return nil, errors.New("no apiVersion in document")
	}

	if v == "troubleshoot.sh/v1beta2" {
		return doc, nil
	}

	if v == "troubleshoot.sh/v1beta3" {
		// For v1beta3, just change the apiVersion to v1beta2
		// The actual template rendering will be handled elsewhere
		parsed["apiVersion"] = "troubleshoot.sh/v1beta2"
		newDoc, err := yaml.Marshal(parsed)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal new spec")
		}
		return newDoc, nil
	}

	if v != "troubleshoot.replicated.com/v1beta1" {
		return nil, errors.Errorf("cannot convert %s", v)
	}

	parsed["apiVersion"] = "troubleshoot.sh/v1beta2"
	newDoc, err := yaml.Marshal(parsed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal new spec")
	}

	return newDoc, nil
}

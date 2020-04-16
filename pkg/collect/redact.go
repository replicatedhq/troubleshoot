package collect

import (
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte, additionalRedactors []*troubleshootv1beta1.Redact) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range input {
		if v != nil {
			redacted, err := redact.Redact(v, k, additionalRedactors)
			if err != nil {
				return nil, err
			}
			result[k] = redacted
		}
	}
	return result, nil
}

package collect

import (
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range input {
		if v != nil {
			redacted, err := redact.Redact(v)
			if err != nil {
				return nil, err
			}
			result[k] = redacted
		}
	}
	return result, nil
}

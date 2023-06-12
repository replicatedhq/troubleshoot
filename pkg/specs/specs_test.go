package specs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

func Test_SplitTroubleshootSecretLabelSelector(t *testing.T) {
	tests := []struct {
		name              string
		selectorString    string
		expectedSelectors []string
		expectedError     bool
	}{
		{
			name:           "Split both troubleshoot and non-troubleshoot labels",
			selectorString: "troubleshoot.io/kind=support-bundle,troubleshoot.sh/kind=support-bundle,a=b",
			expectedSelectors: []string{
				"a=b,troubleshoot.io/kind=support-bundle",
				"a=b,troubleshoot.sh/kind=support-bundle",
			},
			expectedError: false,
		},
		{
			name:              "Split only troubleshoot.io label",
			selectorString:    "troubleshoot.io/kind=support-bundle",
			expectedSelectors: []string{"troubleshoot.io/kind=support-bundle"},
			expectedError:     false,
		},
		{
			name:              "Split only troubleshoot.sh label",
			selectorString:    "troubleshoot.sh/kind=support-bundle",
			expectedSelectors: []string{"troubleshoot.sh/kind=support-bundle"},
			expectedError:     false,
		},
		{
			name:              "Split only non-troubleshoot label",
			selectorString:    "a=b",
			expectedSelectors: []string{"a=b"},
			expectedError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := labels.Parse(tt.selectorString)
			if err != nil {
				t.Errorf("Error parsing selector string: %v", err)
				return
			}

			gotSelectors, err := SplitTroubleshootSecretLabelSelector(nil, selector)
			if (err != nil) != tt.expectedError {
				t.Errorf("Expected error: %v, got: %v", tt.expectedError, err)
				return
			}

			assert.ElementsMatch(t, tt.expectedSelectors, gotSelectors)
		})
	}
}

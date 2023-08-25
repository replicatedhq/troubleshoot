package specs

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
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
		{
			name:              "No selector labels to split",
			selectorString:    "",
			expectedSelectors: []string{},
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

			gotSelectors, err := SplitTroubleshootSecretLabelSelector(context.TODO(), selector)
			if (err != nil) != tt.expectedError {
				t.Errorf("Expected error: %v, got: %v", tt.expectedError, err)
				return
			}

			assert.ElementsMatch(t, tt.expectedSelectors, gotSelectors)
		})
	}
}

func TestLoadFromCluster(t *testing.T) {
	theRedactor := troubleshootv1beta2.Redactor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "Redactor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "redact-some-content",
		},
		Spec: troubleshootv1beta2.RedactorSpec{
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "redact-text-1",
					Removals: troubleshootv1beta2.Removals{
						Values: []string{"TEXT"},
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		selectors []string
		namespace string
		objects   []runtime.Object
		want      *loader.TroubleshootKinds
	}{
		{
			name: "no selectors",
			want: loader.NewTroubleshootKinds(),
		},
		{
			name:      "spec in secret and default label selector",
			namespace: "bigbank",
			selectors: []string{
				"troubleshoot.sh/kind=support-bundle",
			},
			objects: []runtime.Object{
				secretObject("bigbank", map[string]string{
					"troubleshoot.io/kind": "support-bundle",
				}),
			},
			want: &loader.TroubleshootKinds{
				RedactorsV1Beta2: []troubleshootv1beta2.Redactor{theRedactor},
			},
		},
		{
			name:      "spec in secret and no selector argument passed",
			namespace: "bigbank",
			objects: []runtime.Object{
				secretObject("bigbank", map[string]string{
					"troubleshoot.io/kind": "support-bundle",
				}),
			},
			want: loader.NewTroubleshootKinds(),
		},
		{
			name:      "multiple specs default selector",
			namespace: "bigbank",
			objects: []runtime.Object{
				secretObject("bigbank", map[string]string{
					"troubleshoot.io/kind": "support-bundle",
				}),
				secretObject("bigbank", map[string]string{
					"troubleshoot.io/kind": "support-bundle",
				}),
			},
			want: &loader.TroubleshootKinds{
				RedactorsV1Beta2: []troubleshootv1beta2.Redactor{theRedactor, theRedactor},
			},
		},
		{
			name:      "spec in secret but different namespace",
			namespace: "bigbank",
			objects: []runtime.Object{
				secretObject("anotherbank", map[string]string{
					"troubleshoot.io/kind": "support-bundle",
				}),
			},
			want: loader.NewTroubleshootKinds(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset(tt.objects...)
			got, err := LoadFromCluster(ctx, client, tt.selectors, tt.namespace)
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", testutils.AsJSON(t, got), testutils.AsJSON(t, tt.want))
			}
		})
	}
}

func secretObject(ns string, selectors map[string]string) runtime.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("secret-name-%s", uuid.New().String()),
			Namespace: ns,
			Labels:    selectors,
		},
		Data: map[string][]byte{
			"redactor-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redact-some-content
spec:
  redactors:
    - name: redact-text-1
      removals:
        values:
          - TEXT`),
		},
	}
}

package collect

import (
	"testing"
)

func TestExtractSearchFromFQDN(t *testing.T) {
	tests := []struct {
		fqdn     string
		name     string
		expected string
	}{
		{"foo.com.", "foo.com", ""},
		{"bar.com", "bar.com", ""},
		{"*.foo.testcluster.net.", "*", "foo.testcluster.net"},
	}

	for _, test := range tests {
		t.Run(test.fqdn, func(t *testing.T) {
			result := extractSearchFromFQDN(test.fqdn, test.name)
			if result != test.expected {
				t.Errorf("extractSearchFromFQDN(%q, %q) = %q; want %q", test.fqdn, test.name, result, test.expected)
			}
		})
	}
}

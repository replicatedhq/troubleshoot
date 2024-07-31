package collect

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetEtcdArgsByDistribution(t *testing.T) {
	tests := []struct {
		distribution string
		expectedArgs []string
		expectedPath string
		expectedErr  error
	}{
		{
			distribution: "k0s",
			expectedArgs: []string{
				"--cacert", "/var/lib/k0s/pki/etcd/ca.crt",
				"--cert", "/var/lib/k0s/pki/etcd/peer.crt",
				"--key", "/var/lib/k0s/pki/etcd/peer.key",
				"--write-out", "json",
				"--endpoints", "https://127.0.0.1:2379",
			},
			expectedPath: "/var/lib/k0s/pki/etcd",
			expectedErr:  nil,
		},
		{
			distribution: "embedded-cluster",
			expectedArgs: []string{
				"--cacert", "/var/lib/k0s/pki/etcd/ca.crt",
				"--cert", "/var/lib/k0s/pki/etcd/peer.crt",
				"--key", "/var/lib/k0s/pki/etcd/peer.key",
				"--write-out", "json",
				"--endpoints", "https://127.0.0.1:2379",
			},
			expectedPath: "/var/lib/k0s/pki/etcd",
			expectedErr:  nil,
		},
		{
			distribution: "kurl",
			expectedArgs: []string{
				"--cacert", "/etc/kubernetes/pki/etcd/ca.crt",
				"--cert", "/etc/kubernetes/pki/etcd/healthcheck-client.crt",
				"--key", "/etc/kubernetes/pki/etcd/healthcheck-client.key",
				"--write-out", "json",
				"--endpoints", "https://127.0.0.1:2379",
			},
			expectedPath: "/etc/kubernetes/pki/etcd",
			expectedErr:  nil,
		},
		{
			distribution: "unknown",
			expectedArgs: nil,
			expectedPath: "",
			expectedErr:  errors.Errorf("distribution unknown not supported"),
		},
	}

	for _, test := range tests {
		args, path, err := getEtcdArgsByDistribution(test.distribution)
		assert.Equal(t, test.expectedArgs, args)
		assert.Equal(t, test.expectedPath, path)
		if test.expectedErr != nil {
			assert.NotNil(t, err)
			assert.EqualError(t, test.expectedErr, err.Error())
		}
	}
}

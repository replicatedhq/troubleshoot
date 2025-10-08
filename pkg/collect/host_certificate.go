package collect

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"path/filepath"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const KeyPairMissing = "key-pair-missing"
const KeyPairSwitched = "key-pair-switched"
const KeyPairEncrypted = "key-pair-encrypted"
const KeyPairMismatch = "key-pair-mismatch"
const KeyPairInvalid = "key-pair-invalid"
const KeyPairValid = "key-pair-valid"

type CollectHostCertificate struct {
	hostCollector *troubleshootv1beta2.Certificate
	BundlePath    string
}

func (c *CollectHostCertificate) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Certificate Key Pair")
}

func (c *CollectHostCertificate) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCertificate) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	var result = KeyPairValid
	var collectorErr error

	_, err := tls.LoadX509KeyPair(c.hostCollector.CertificatePath, c.hostCollector.KeyPath)
	if err != nil {
		collectorErr = err
		if strings.Contains(err.Error(), "no such file") {
			result = KeyPairMissing
		} else if strings.Contains(err.Error(), "PEM inputs may have been switched") {
			result = KeyPairSwitched
		} else if strings.Contains(err.Error(), "found a certificate rather than a key") {
			result = KeyPairSwitched
		} else if strings.Contains(err.Error(), "private key does not match public key") {
			result = KeyPairMismatch
		} else if strings.Contains(err.Error(), "failed to parse private key") {
			if encrypted, _ := isEncryptedKey(c.hostCollector.KeyPath); encrypted {
				result = KeyPairEncrypted
			} else {
				result = KeyPairInvalid
			}
		} else {
			result = KeyPairInvalid
		}
	}

	b := []byte(result)

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "certificate"
	}
	name := filepath.Join("host-collectors/certificate", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, collectorErr
}

func isEncryptedKey(filename string) (bool, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}
	return bytes.Contains(data, []byte("ENCRYPTED")), nil
}

func (c *CollectHostCertificate) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}

package collect

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"path/filepath"
	"strings"
)

const KeyPairMissing = "key-pair-missing"
const KeyPairSwitched = "key-pair-switched"
const KeyPairEncrypted = "key-pair-encrypted"
const KeyPairMismatch = "key-pair-mismatch"
const KeyPairInvalid = "key-pair-invalid"
const KeyPairValid = "key-pair-valid"

func HostCertificate(c *HostCollector) (map[string][]byte, error) {
	var result = KeyPairValid

	_, err := tls.LoadX509KeyPair(c.Collect.Certificate.CertificatePath, c.Collect.Certificate.KeyPath)
	if err != nil {
		if strings.Contains(err.Error(), "no such file") {
			result = KeyPairMissing
		} else if strings.Contains(err.Error(), "PEM inputs may have been switched") {
			result = KeyPairSwitched
		} else if strings.Contains(err.Error(), "found a certificate rather than a key") {
			result = KeyPairSwitched
		} else if strings.Contains(err.Error(), "private key does not match public key") {
			result = KeyPairMismatch
		} else if strings.Contains(err.Error(), "failed to parse private key") {
			if encrypted, _ := isEncryptedKey(c.Collect.Certificate.KeyPath); encrypted {
				result = KeyPairEncrypted
			} else {
				result = KeyPairInvalid
			}
		} else {
			result = KeyPairInvalid
		}
	}

	collectorName := c.Collect.Certificate.CollectorName
	if collectorName == "" {
		collectorName = "certificate"
	}
	name := filepath.Join("certificate", collectorName+".json")

	return map[string][]byte{
		name: []byte(result),
	}, nil
}

func isEncryptedKey(filename string) (bool, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}
	return bytes.Contains(data, []byte("ENCRYPTED")), nil
}

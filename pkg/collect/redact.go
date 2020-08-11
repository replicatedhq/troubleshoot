package collect

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"path"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte, additionalRedactors []*troubleshootv1beta1.Redact) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range input {
		if v == nil {
			continue
		}
		if path.Ext(string(k)) == ".tar" || path.Ext(string(k)) == ".tar.gz" {
			tarFile := bytes.NewBuffer(v)
			buff := new(bytes.Buffer)
			unRedacted, fileHeaders, _ := untarFile(tarFile, k+strings.TrimSuffix(path.Base(k), ".tar"))

			files, _ := redactMap(unRedacted, additionalRedactors)
			tw := tar.NewWriter(buff)
			for p, f := range files {
				fileHeaders[p].Size = int64(binary.Size(f))
				tw.WriteHeader(fileHeaders[p])
				tw.Write(f)
			}
			tw.Close()
			result[k] = buff.Bytes()

			continue
		}
		redacted, err := redact.Redact(v, k, additionalRedactors)
		if err != nil {
			return nil, err
		}
		result[k] = redacted
	}
	return result, nil
}

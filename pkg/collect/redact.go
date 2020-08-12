package collect

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"io"
	"path"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte, additionalRedactors []*troubleshootv1beta1.Redact) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range input {
		if v == nil {
			continue
		}
		if path.Ext(k) == ".tar" {
			tarFile := bytes.NewBuffer(v)
			buff := new(bytes.Buffer)
			unRedacted, fileHeaders, err := untarFile(tarFile)
			if err != nil {
				return nil, err
			}
			files, err := redactMap(unRedacted, additionalRedactors)
			if err != nil {
				return nil, err
			}
			tw := tar.NewWriter(buff)
			for p, f := range files {
				//File size must be recalculated in case the redactor added some bytes when redacting.
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

func untarFile(tarFile *bytes.Buffer) (map[string][]byte, map[string]*tar.Header, error) {
	tarReader := tar.NewReader(tarFile)
	fileHeaders := make(map[string]*tar.Header)
	files := make(map[string][]byte)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return nil, nil, err
			}
			break
		}
		if header.FileInfo().IsDir() {
			continue
		}
		file := new(bytes.Buffer)
		io.Copy(file, tarReader)
		files[header.Name] = file.Bytes()
		fileHeaders[header.Name] = header
	}
	return files, fileHeaders, nil
}

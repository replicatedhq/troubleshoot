package collect

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"io"
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte, additionalRedactors []*troubleshootv1beta1.Redact) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range input {
		if v == nil {
			continue
		}
		//If the file is a .tar file, it must not be redacted. Instead it is decompressed and each file inside the
		//tar is decompressed, redacted and compressed back into the tar.
		if filepath.Ext(k) == ".tar" {
			tarFile := bytes.NewBuffer(v)
			unRedacted, tarHeaders, err := untarFile(tarFile)
			if err != nil {
				return nil, err
			}
			redacted, err := redactMap(unRedacted, additionalRedactors)
			if err != nil {
				return nil, err
			}
			result[k], err = tarFiles(redacted, tarHeaders)
			if err != nil {
				return nil, err
			}
			//Content of the tar file was redacted. Continue to next file.
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

func tarFiles(tarContent map[string][]byte, tarHeaders map[string]*tar.Header) ([]byte, error) {
	buff := new(bytes.Buffer)
	tw := tar.NewWriter(buff)
	defer tw.Close()
	var err error
	for p, f := range tarContent {
		if tarHeaders[p].FileInfo().IsDir() {
			err = tw.WriteHeader(tarHeaders[p])
			if err != nil {
				return nil, err
			}
			continue
		}
		//File size must be recalculated in case the redactor added some bytes when redacting.
		tarHeaders[p].Size = int64(binary.Size(f))
		err = tw.WriteHeader(tarHeaders[p])
		if err != nil {
			return nil, err
		}
		_, err = tw.Write(f)
		if err != nil {
			return nil, err
		}
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil

}

func untarFile(tarFile *bytes.Buffer) (map[string][]byte, map[string]*tar.Header, error) {
	tarReader := tar.NewReader(tarFile)
	tarHeaders := make(map[string]*tar.Header)
	tarContent := make(map[string][]byte)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return nil, nil, err
			}
			break
		}
		file := new(bytes.Buffer)
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return nil, nil, err
		}
		tarContent[header.Name] = file.Bytes()
		tarHeaders[header.Name] = header

	}
	return tarContent, tarHeaders, nil
}

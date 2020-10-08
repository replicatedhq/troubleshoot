package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"path/filepath"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactMap(input map[string][]byte, additionalRedactors []*troubleshootv1beta2.Redact, analyzers []*troubleshootv1beta2.Analyze) (map[string][]byte, map[string][]byte, error) {
	result := make(map[string][]byte)
	protected := make(map[string][]byte)
	for k, v := range input {
		if v == nil {
			continue
		}
		//If the file is .tar, .tgz or .tar.gz, it must not be redacted. Instead it is decompressed and each file inside the
		//tar is decompressed, redacted and compressed back into the tar.
		if filepath.Ext(k) == ".tar" || filepath.Ext(k) == ".tgz" || strings.HasSuffix(k, ".tar.gz") {
			tarFile := bytes.NewBuffer(v)
			unRedacted, tarHeaders, err := decompressFile(tarFile, k)
			if err != nil {
				return nil, nil, err
			}
			redacted, _, err := redactMap(unRedacted, additionalRedactors, analyzers)
			if err != nil {
				return nil, nil, err
			}
			result[k], err = compressFiles(redacted, tarHeaders, k)
			if err != nil {
				return nil, nil, err
			}
			//Content of the tar file was redacted. Continue to next file.
			continue
		}
		if strings.Contains(k, "cluster-resources/image-pull-secrets/") {
			if analyzers != nil {
				for _, analyzer := range analyzers {
					if analyzer.ImagePullSecret != nil {
						protected[k] = v
					}
				}
			}
		}
		redacted, err := redact.Redact(v, k, additionalRedactors)
		if err != nil {
			return nil, nil, err
		}
		result[k] = redacted
	}
	return result, protected, nil
}

func compressFiles(tarContent map[string][]byte, tarHeaders map[string]*tar.Header, filename string) ([]byte, error) {
	buff := new(bytes.Buffer)
	var tw *tar.Writer
	var zw *gzip.Writer
	if filepath.Ext(filename) != ".tar" {
		zw = gzip.NewWriter(buff)
		tw = tar.NewWriter(zw)
		defer zw.Close()
	} else {
		tw = tar.NewWriter(buff)
	}
	defer tw.Close()
	for p, f := range tarContent {
		if tarHeaders[p].FileInfo().IsDir() {
			err := tw.WriteHeader(tarHeaders[p])
			if err != nil {
				return nil, err
			}
			continue
		}
		//File size must be recalculated in case the redactor added some bytes while redacting.
		tarHeaders[p].Size = int64(binary.Size(f))
		err := tw.WriteHeader(tarHeaders[p])
		if err != nil {
			return nil, err
		}
		_, err = tw.Write(f)
		if err != nil {
			return nil, err
		}
	}
	err := tw.Close()
	if err != nil {
		return nil, err
	}
	if filepath.Ext(filename) != ".tar" {
		err = zw.Close()
		if err != nil {
			return nil, err
		}
	}
	return buff.Bytes(), nil

}

func decompressFile(tarFile *bytes.Buffer, filename string) (map[string][]byte, map[string]*tar.Header, error) {
	var tarReader *tar.Reader
	var zr *gzip.Reader
	var err error
	if filepath.Ext(filename) != ".tar" {
		zr, err = gzip.NewReader(tarFile)
		if err != nil {
			return nil, nil, err
		}
		defer zr.Close()
		tarReader = tar.NewReader(zr)
	} else {
		tarReader = tar.NewReader(tarFile)
	}
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

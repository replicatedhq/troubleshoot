package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func redactResult(bundlePath string, input CollectorResult, additionalRedactors []*troubleshootv1beta2.Redact) error {
	for k, v := range input {
		var reader io.Reader
		if v == nil {
			r, err := input.GetReader(bundlePath, k)
			if err != nil {
				if os.IsNotExist(errors.Cause(err)) {
					continue
				}
				return errors.Wrap(err, "failed to get reader")
			}
			if r, ok := r.(io.ReadCloser); ok {
				defer r.Close()
			}

			reader = r
		} else {
			reader = bytes.NewBuffer(v)
		}

		//If the file is .tar, .tgz or .tar.gz, it must not be redacted. Instead it is decompressed and each file inside the
		//tar is decompressed, redacted and compressed back into the tar.
		if filepath.Ext(k) == ".tar" || filepath.Ext(k) == ".tgz" || strings.HasSuffix(k, ".tar.gz") {
			tmpDir, err := ioutil.TempDir("", "troubleshoot-subresult-")
			if err != nil {
				return errors.Wrap(err, "failed to create temp dir")
			}
			defer os.RemoveAll(tmpDir)

			subResult, tarHeaders, err := decompressFile(tmpDir, reader, k)
			if err != nil {
				return errors.Wrap(err, "failed to decompress file")
			}
			err = redactResult(tmpDir, subResult, additionalRedactors)
			if err != nil {
				return errors.Wrap(err, "failed to redact file")
			}

			dstFilename := filepath.Join(bundlePath, k)
			err = compressFiles(tmpDir, subResult, tarHeaders, dstFilename)
			if err != nil {
				return errors.Wrap(err, "failed to re-compress file")
			}

			os.RemoveAll(tmpDir) // ensure clean up on each iteration in addition to the defer

			//Content of the tar file was redacted. Continue to next file.
			continue
		}

		redacted, err := redact.Redact(reader, k, additionalRedactors)
		if err != nil {
			return errors.Wrap(err, "failed to redact")
		}

		err = input.ReplaceResult(bundlePath, k, redacted)
		if err != nil {
			return errors.Wrap(err, "failed to create redacted result")
		}
	}
	return nil
}

func compressFiles(bundlePath string, result CollectorResult, tarHeaders map[string]*tar.Header, dstFilename string) error {
	fw, err := os.Create(dstFilename)
	if err != nil {
		return errors.Wrap(err, "failed to open destination file")
	}
	defer fw.Close()

	var tw *tar.Writer
	var zw *gzip.Writer
	if filepath.Ext(dstFilename) != ".tar" {
		zw = gzip.NewWriter(fw)
		tw = tar.NewWriter(zw)
		defer zw.Close()
	} else {
		tw = tar.NewWriter(fw)
	}
	defer tw.Close()

	for subPath := range result {
		if tarHeaders[subPath].FileInfo().IsDir() {
			err := tw.WriteHeader(tarHeaders[subPath])
			if err != nil {
				return err
			}
			continue
		}

		srcFilename := filepath.Join(bundlePath, subPath)
		fileStat, err := os.Stat(srcFilename)
		if err != nil {
			return errors.Wrap(err, "failed to stat source file")
		}

		fr, err := os.Open(srcFilename)
		if err != nil {
			return errors.Wrap(err, "failed to open source file")
		}
		defer fr.Close()

		//File size must be recalculated in case the redactor added some bytes while redacting.
		tarHeaders[subPath].Size = fileStat.Size()
		err = tw.WriteHeader(tarHeaders[subPath])
		if err != nil {
			return errors.Wrap(err, "failed to write tar header")
		}
		_, err = io.Copy(tw, fr)
		if err != nil {
			return errors.Wrap(err, "failed to write tar data")
		}

		_ = fr.Close()
	}

	err = tw.Close()
	if err != nil {
		return err
	}
	if filepath.Ext(dstFilename) != ".tar" {
		err = zw.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func decompressFile(dstDir string, tarFile io.Reader, filename string) (CollectorResult, map[string]*tar.Header, error) {
	result := NewResult()

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
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return nil, nil, err
			}
			break
		}
		if !header.FileInfo().Mode().IsRegular() {
			continue
		}
		result.SaveResult(dstDir, header.Name, tarReader)
		tarHeaders[header.Name] = header

	}

	return result, tarHeaders, nil
}

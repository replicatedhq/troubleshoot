package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
	"k8s.io/klog/v2"
)

func RedactResult(bundlePath string, input CollectorResult, additionalRedactors []*troubleshootv1beta2.Redact) error {
	processedSymlinks := make(map[string]bool)

	for k, v := range input {
		file := k

		var reader io.Reader
		if v == nil {
			// Check if file has been processed already
			if processedSymlinks[file] {
				continue
			}

			// Collected contents are in a file. Get a reader to the file.
			fullPath := filepath.Join(bundlePath, file)
			info, err := os.Lstat(fullPath)
			if err != nil {
				if os.IsNotExist(errors.Cause(err)) {
					// File not found, moving on.
					continue
				}
				return errors.Wrap(err, "failed to stat file")
			}

			// Redact the target file of a symlink
			// There is an opportunity for improving performance here by skipping symlinks
			// if a target has been redacted already, but that would require
			// some extra logic to ensure that a spec filtering only symlinks still works.
			if info.Mode().Type() == os.ModeSymlink {
				symlink := file
				target, err := os.Readlink(fullPath)
				if err != nil {
					return errors.Wrap(err, "failed to read symlink")
				}

				// Mark symlink target as processed
				processedSymlinks[target] = true

				// Get the relative path to the target file to conform with
				// the path formats of the CollectorResult
				file, err = filepath.Rel(bundlePath, target)
				if err != nil {
					return errors.Wrap(err, "failed to get relative path")
				}
				klog.V(2).Infof("Redacting %s (symlink => %s)\n", file, symlink)
			} else {
				klog.V(2).Infof("Redacting %s\n", file)
			}
			r, err := input.GetReader(bundlePath, file)
			if err != nil {
				if os.IsNotExist(errors.Cause(err)) {
					continue
				}
				return errors.Wrap(err, "failed to get reader")
			}
			defer r.Close()

			reader = r
		} else {
			// Collected contents are in memory. Get a reader to the memory buffer.
			reader = bytes.NewBuffer(v)
		}

		// If the file is .tar, .tgz or .tar.gz, it must not be redacted. Instead it is
		// decompressed and each file inside the tar redacted and compressed back into the archive.
		if filepath.Ext(file) == ".tar" || filepath.Ext(file) == ".tgz" || strings.HasSuffix(file, ".tar.gz") {
			tmpDir, err := os.MkdirTemp("", "troubleshoot-subresult-")
			if err != nil {
				return errors.Wrap(err, "failed to create temp dir")
			}
			defer os.RemoveAll(tmpDir)

			subResult, tarHeaders, err := decompressFile(tmpDir, reader, file)
			if err != nil {
				return errors.Wrap(err, "failed to decompress file")
			}
			err = RedactResult(tmpDir, subResult, additionalRedactors)
			if err != nil {
				return errors.Wrap(err, "failed to redact file")
			}

			dstFilename := filepath.Join(bundlePath, file)
			err = compressFiles(tmpDir, subResult, tarHeaders, dstFilename)
			if err != nil {
				return errors.Wrap(err, "failed to re-compress file")
			}

			os.RemoveAll(tmpDir) // ensure clean up on each iteration in addition to the defer

			//Content of the tar file was redacted. Continue to next file.
			continue
		}

		redacted, err := redact.Redact(reader, file, additionalRedactors)
		if err != nil {
			return errors.Wrap(err, "failed to redact io stream")
		}

		err = input.ReplaceResult(bundlePath, file, redacted)
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

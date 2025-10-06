package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
	"k8s.io/klog/v2"
)

// Max number of concurrent redactors to run
// Ensure the number is low enough since each of the redactors
// also spawns goroutines to redact files in tar archives and
// other goroutines for each redactor spec.
const MAX_CONCURRENT_REDACTORS = 10

func RedactResult(bundlePath string, input CollectorResult, additionalRedactors []*troubleshootv1beta2.Redact) error {
	wg := &sync.WaitGroup{}

	// Error channel to capture errors from goroutines
	errorCh := make(chan error, len(input))
	limitCh := make(chan struct{}, MAX_CONCURRENT_REDACTORS)
	defer close(limitCh)

	for k, v := range input {
		limitCh <- struct{}{}

		wg.Add(1)

		go func(file string, data []byte) {
			defer wg.Done()
			defer func() { <-limitCh }() // free up after the function execution has run

			var reader io.Reader
			var readerCloseFn func() error // Function to close reader if needed
			if data == nil {

				// Collected contents are in a file. Get a reader to the file.
				info, err := os.Lstat(filepath.Join(bundlePath, file))
				if err != nil {
					if os.IsNotExist(errors.Cause(err)) {
						// File not found, moving on.
						return
					}
					errorCh <- errors.Wrap(err, "failed to stat file")
					return
				}

				// Redact the target file of a symlink
				// There is an opportunity for improving performance here by skipping symlinks
				// if a target has been redacted already, but that would require
				// some extra logic to ensure that a spec filtering only symlinks still works.
				if info.Mode().Type() == os.ModeSymlink {
					symlink := file
					target, err := os.Readlink(filepath.Join(bundlePath, symlink))
					if err != nil {
						errorCh <- errors.Wrap(err, "failed to read symlink")
						return
					}
					// Get the relative path to the target file to conform with
					// the path formats of the CollectorResult
					file, err = filepath.Rel(bundlePath, target)
					if err != nil {
						errorCh <- errors.Wrap(err, "failed to get relative path")
						return
					}
					klog.V(4).Infof("Redacting %s (symlink => %s)\n", file, symlink)
				} else {
					klog.V(4).Infof("Redacting %s\n", file)
				}
				r, err := input.GetReader(bundlePath, file)
				if err != nil {
					if os.IsNotExist(errors.Cause(err)) {
						return
					}
					errorCh <- errors.Wrap(err, "failed to get reader")
					return
				}

				reader = r
				readerCloseFn = r.Close // Ensure we close the file later
			} else {
				// Collected contents are in memory. Get a reader to the memory buffer.
				reader = bytes.NewBuffer(data)
				readerCloseFn = func() error { return nil } // No-op for in-memory data
			}

			// Ensure the reader is eventually closed even on error paths.
			// This defer is guarded by setting readerCloseFn to nil after any explicit close
			// to prevent double-closing (notably when we must close before rewriting files on Windows).
			defer func() {
				if readerCloseFn != nil {
					if err := readerCloseFn(); err != nil {
						klog.Warningf("Failed to close reader for %s: %v", file, err)
					}
				}
			}()

			// If the file is .tar, .tgz or .tar.gz, it must not be redacted. Instead it is
			// decompressed and each file inside the tar redacted and compressed back into the archive.
			if filepath.Ext(file) == ".tar" || filepath.Ext(file) == ".tgz" || strings.HasSuffix(file, ".tar.gz") {
				tmpDir, err := os.MkdirTemp("", "troubleshoot-subresult-")
				if err != nil {
					errorCh <- errors.Wrap(err, "failed to create temp dir")
					return
				}
				defer os.RemoveAll(tmpDir)

				subResult, tarHeaders, err := decompressFile(tmpDir, reader, file)
				if err != nil {
					errorCh <- errors.Wrap(err, "failed to decompress file")
					return
				}

				// Close the reader before we write back to the same file path (Windows safety)
				if err := readerCloseFn(); err != nil {
					klog.Warningf("Failed to close reader for %s: %v", file, err)
					errorCh <- errors.Wrap(err, "failed to close reader")
					return
				}
				readerCloseFn = nil

				err = RedactResult(tmpDir, subResult, additionalRedactors)
				if err != nil {
					errorCh <- errors.Wrap(err, "failed to redact file")
					return
				}

				dstFilename := filepath.Join(bundlePath, file)
				err = compressFiles(tmpDir, subResult, tarHeaders, dstFilename)
				if err != nil {
					errorCh <- errors.Wrap(err, "failed to re-compress file")
					return
				}

				os.RemoveAll(tmpDir) // ensure clean up on each iteration in addition to the defer

				//Content of the tar file was redacted. return to next file.
				return
			}

			redacted, err := redact.Redact(reader, file, additionalRedactors)
			if err != nil {
				errorCh <- errors.Wrap(err, "failed to redact io stream")
				return
			}

			// Fully consume the redacted reader into a buffer while the source file is still open
			// This is required on Windows where we can't delete a file that's open
			var redactedBuf bytes.Buffer
			_, err = io.Copy(&redactedBuf, redacted)
			if err != nil {
				errorCh <- errors.Wrap(err, "failed to read redacted data")
				return
			}

			// Close the reader now that we've consumed all the data (Windows safety)
			if err := readerCloseFn(); err != nil {
				klog.Warningf("Failed to close reader for %s: %v", file, err)
				errorCh <- errors.Wrap(err, "failed to close reader")
				return
			}
			readerCloseFn = nil

			// Now replace the file with the buffered redacted content
			err = input.ReplaceResult(bundlePath, file, &redactedBuf)
			if err != nil {
				errorCh <- errors.Wrap(err, "failed to create redacted result")
				return
			}
		}(k, v)
	}

	go func() {
		wg.Wait()
		close(errorCh)
	}()

	for err := range errorCh {
		if err != nil {
			return err
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

package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type CollectorResult map[string][]byte

func NewResult() CollectorResult {
	return map[string][]byte{}
}

// SymLinkResult creates a symlink (relativeLinkPath) of relativeFilePath in the bundle. If bundlePath
// is empty, no symlink is created. The relativeLinkPath is always saved in the result map.
func (r CollectorResult) SymLinkResult(bundlePath, relativeLinkPath, relativeFilePath string) error {
	// We should have saved the result this symlink is pointing to prior to creating it
	klog.V(4).Info("Creating symlink ", relativeLinkPath, " -> ", relativeFilePath)
	data, ok := r[relativeFilePath]
	if !ok {
		return errors.Errorf("cannot create symlink, result in %q not found", relativeFilePath)
	}

	if bundlePath == "" {
		// Memory only bundle
		r[relativeLinkPath] = data
		return nil
	}

	linkPath := filepath.Join(bundlePath, relativeLinkPath)
	filePath := filepath.Join(bundlePath, relativeFilePath)

	// If both paths are the same, don't create a symlink
	if linkPath == filePath {
		return nil
	}

	linkDirPath := filepath.Dir(linkPath)

	// Create the directory for the symlink if it doesn't exist
	err := os.MkdirAll(linkDirPath, 0777)
	if err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	// Ensure the file exists
	_, err = os.Stat(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to stat. File may not exist")
	}

	// Do nothing if the symlink already exists
	_, err = os.Lstat(linkPath)
	if err == nil {
		return nil
	}

	// Create the symlink
	// NOTE: When creating an archive, relative paths are used
	// to make the bundle more portable. That implementation
	// lives in CollectorResultFromBundle function. This path needs to
	// remain as-is to support memory only bundles e.g preflight
	err = os.Symlink(filePath, linkPath)
	if err != nil {
		return errors.Wrap(err, "failed to create symlink")
	}

	klog.V(4).Infof("Added %q symlink of %q in bundle output", relativeLinkPath, relativeFilePath)
	// store the file name referencing the symlink to have archived
	r[relativeLinkPath] = nil

	return nil
}

// AddResult combines another results object into this collector result.
// This ensures when archiving a bundle from the result, all files are included.
// It also ensures that when operating on the results in memory (e.g preflights),
// all files are included.
func (r CollectorResult) AddResult(other CollectorResult) {
	for k, v := range other {
		r[k] = v
	}
}

// SaveResult saves the collector result to relativePath file on disk. If bundlePath is
// empty, no file is created on disk. The relativePath is always saved in the result map.
func (r CollectorResult) SaveResult(bundlePath string, relativePath string, reader io.Reader) error {
	if reader == nil {
		return nil
	}

	if bundlePath == "" {
		data, err := io.ReadAll(reader)
		if err != nil {
			return errors.Wrap(err, "failed to read data")
		}
		// Memory only bundle
		klog.V(4).Infof("Added %q to bundle output", relativePath)
		r[relativePath] = data
		return nil
	}

	r[relativePath] = nil // save the file name referencing the file on disk

	fileDir, fileName := filepath.Split(relativePath)
	outPath := filepath.Join(bundlePath, fileDir)

	if err := os.MkdirAll(outPath, 0777); err != nil {
		return errors.Wrap(err, "failed to create output file directory")
	}

	f, err := os.Create(filepath.Join(outPath, fileName))
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		return errors.Wrap(err, "failed to copy data")
	}

	fileInfo, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to stat file")
	}

	klog.V(4).Infof("Added %q (%d KB) to bundle output", relativePath, fileInfo.Size()/(1024))
	return nil
}

// SaveResults walk a target directory and call SaveResult on all files retrieved from the walk.
func (r CollectorResult) SaveResults(bundlePath, relativePath, targetDir string) error {
	dirPath := path.Join(bundlePath, relativePath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return errors.Wrap(err, "failed to create output file directory")
	}

	err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return errors.Wrap(err, "error from WalkDirFunc")
		}

		if !d.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to open file: %s", path))
			}
			fileBytes, err := io.ReadAll(file)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to read file: %s", path))
			}
			bundleRelativePath := filepath.Join(relativePath, strings.TrimPrefix(path, targetDir+"/"))
			err = r.SaveResult(bundlePath, bundleRelativePath, bytes.NewBuffer(fileBytes))
			if err != nil {
				return errors.Wrap(err, "error from SaveResult call")
			}
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "error from WalkDir call")
	}

	return nil
}

func (r CollectorResult) ReplaceResult(bundlePath string, relativePath string, reader io.Reader) error {
	if bundlePath == "" {
		data, err := io.ReadAll(reader)
		if err != nil {
			return errors.Wrap(err, "failed to read data")
		}
		// Memory only bundle
		r[relativePath] = data
		return nil
	}

	// Create temp file in DESTINATION directory to prevent cross-device issues
	// This follows the same pattern as pkg/updater/updater.go for Windows compatibility
	finalPath := filepath.Join(bundlePath, relativePath)
	destDir := filepath.Dir(finalPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create destination directory")
	}

	// Create temporary file in destination directory (not system temp)
	tmpFile, err := os.CreateTemp(destDir, "replace-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}

	// Write data to the temporary file
	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		tmpFile.Close()
		// Clean up temp file on write failure
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			klog.V(2).Infof("Failed to cleanup temp file %s: %v", tmpFile.Name(), removeErr)
		}
		return errors.Wrap(err, "failed to write tmp file")
	}

	// Close the file to ensure all data is written
	if err := tmpFile.Close(); err != nil {
		// Clean up temp file on close failure
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			klog.V(2).Infof("Failed to cleanup temp file %s: %v", tmpFile.Name(), removeErr)
		}
		return errors.Wrap(err, "failed to close tmp file")
	}

	// Use Windows-aware file replacement with retry logic
	if err := replaceFileWithRetry(tmpFile.Name(), finalPath); err != nil {
		// Clean up temp file on replacement failure
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			klog.V(2).Infof("Failed to cleanup temp file %s after replacement failure: %v", tmpFile.Name(), removeErr)
		}
		return errors.Wrap(err, "failed to replace file")
	}

	klog.V(3).Infof("Successfully replaced file: %s", relativePath)
	return nil
}

func (r CollectorResult) GetReader(bundlePath string, relativePath string) (io.ReadCloser, error) {
	if r[relativePath] != nil {
		// Memory only bundle
		return io.NopCloser(bytes.NewReader(r[relativePath])), nil
	}

	if bundlePath == "" {
		return nil, errors.New("cannot create reader, bundle path is empty")
	}

	filename := filepath.Join(bundlePath, relativePath)
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}

	return f, nil
}

func (r CollectorResult) GetWriter(bundlePath string, relativePath string) (io.Writer, error) {
	if bundlePath == "" {
		// Memory only bundle
		var b bytes.Buffer
		return &b, nil
	}

	fileDir, _ := filepath.Split(relativePath)
	outPath := filepath.Join(bundlePath, fileDir)
	if err := os.MkdirAll(outPath, 0777); err != nil {
		return nil, errors.Wrap(err, "failed to create output directory")
	}

	filename := filepath.Join(bundlePath, relativePath)
	f, err := os.Create(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}

	r[relativePath] = nil // save the the file name referencing the file on disk

	return f, nil
}

func (r CollectorResult) CloseWriter(bundlePath string, relativePath string, writer interface{}) error {
	if c, ok := writer.(io.Closer); ok {
		return errors.Wrap(c.Close(), "failed to close writer")
	}

	if buff, ok := writer.(*bytes.Buffer); ok {
		// Memory only bundle
		b := buff.Bytes()
		if b == nil {
			// nil means data is on disk, so make it an empty array
			b = []byte{}
		}
		r[relativePath] = b
		return nil
	}

	return errors.Errorf("cannot close writer of type %T", writer)
}

// ArchiveSupportBundle creates an archive of the files in the bundle directory
// Deprecated: Use better named ArchiveBundle since this method is used to archive any directory
func (r CollectorResult) ArchiveSupportBundle(bundlePath string, outputFilename string) error {
	return r.ArchiveBundle(bundlePath, outputFilename)
}

// ArchiveBundle creates an archive of the files in the bundle directory
func (r CollectorResult) ArchiveBundle(bundlePath string, outputFilename string) error {
	fileWriter, err := os.Create(outputFilename)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer fileWriter.Close()

	gzipWriter := gzip.NewWriter(fileWriter)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for relativeName := range r {
		filename := filepath.Join(bundlePath, relativeName)
		info, err := os.Lstat(filename)
		if err != nil {
			return errors.Wrap(err, "failed to stat file")
		}

		fileMode := info.Mode()
		if !(fileMode.IsRegular() || fileMode.Type() == os.ModeSymlink) {
			// support bundle can have only files or symlinks
			continue
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return errors.Wrap(err, "failed to tar file info header")
		}

		parentDirName := filepath.Dir(bundlePath) // this is to have the files inside a subdirectory
		nameInArchive, err := filepath.Rel(parentDirName, filename)
		if err != nil {
			return errors.Wrap(err, "failed to create relative file name")
		}
		// Use the relative path of the file so as to retain directory hierachy
		hdr.Name = nameInArchive

		if fileMode.Type() == os.ModeSymlink {
			linkTarget, err := os.Readlink(filename)
			if err != nil {
				return errors.Wrap(err, "failed to get symlink target")
			}

			linkTargetInArchive, err := filepath.Rel(parentDirName, linkTarget)
			if err != nil {
				return errors.Wrap(err, "failed to create relative file name")
			}

			// Use the relative path of the link target so as to retain directory hierachy
			// i.e link -> ../../../../target.log. When untarred, the link will point to the
			// relative path of the target file on the machine where it is untarred.
			relLinkPath, err := filepath.Rel(filepath.Dir(nameInArchive), linkTargetInArchive)
			if err != nil {
				return errors.Wrap(err, "failed to create relative path of symlink target file")
			}

			hdr.Linkname = relLinkPath
		}

		err = tarWriter.WriteHeader(hdr)
		if err != nil {
			return errors.Wrap(err, "failed to write tar header")
		}

		func() error {
			if fileMode.Type() == os.ModeSymlink {
				// Don't copy the symlink, just write the header which
				// will create a symlink in the tarball
				klog.V(4).Infof("Added %q symlink to bundle archive", hdr.Linkname)
				return nil
			}

			fileReader, err := os.Open(filename)
			if err != nil {
				return errors.Wrap(err, "failed to open source file")
			}
			defer fileReader.Close()

			_, err = io.Copy(tarWriter, fileReader)
			if err != nil {
				return errors.Wrap(err, "failed to copy file into archive")
			}
			klog.V(4).Infof("Added %q file to bundle archive", hdr.Name)

			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

// CollectorResultFromBundle creates a CollectorResult from a bundle directory
// The bundle directory is not necessarily a support bundle, it can be any directory
// of collected files as part of other operations or files that are already on disk.
func CollectorResultFromBundle(bundleDir string) (CollectorResult, error) {
	// Check directory exists
	if _, err := os.Stat(bundleDir); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "bundle directory does not exist")
	}

	// Walk the directory and add all files to the collector result
	result := make(CollectorResult)
	err := filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(bundleDir, path)
		if err != nil {
			return err
		}

		result[rel] = nil
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk bundle directory")
	}

	return result, nil
}

// TarSupportBundleDir wraps ArchiveSupportBundle for backwards compatibility
// Deprecated: Remove in a future version (v1.0)
func TarSupportBundleDir(bundlePath string, input CollectorResult, outputFilename string) error {
	// Is this used anywhere external anyway?
	return input.ArchiveBundle(bundlePath, outputFilename)
}

// replaceFileWithRetry provides platform-aware file replacement with retry logic for Windows
func replaceFileWithRetry(srcPath, dstPath string) error {
	if runtime.GOOS == "windows" {
		return replaceFileWindows(srcPath, dstPath)
	}
	// Unix/Linux - simple rename
	return os.Rename(srcPath, dstPath)
}

// replaceFileWindows handles Windows-specific file replacement with retry logic for file locking issues
func replaceFileWindows(srcPath, dstPath string) error {
	const maxRetries = 5
	const baseDelay = 50 * time.Millisecond

	klog.V(2).Infof("Windows file replacement: %s -> %s", srcPath, dstPath)

	for attempt := 0; attempt < maxRetries; attempt++ {
		// On Windows, we often need to delete the target file first before rename
		if _, err := os.Stat(dstPath); err == nil {
			// Target file exists, try to delete it first
			if removeErr := os.Remove(dstPath); removeErr != nil {
				klog.V(2).Infof("Failed to remove existing target file: %v", removeErr)
				// Continue anyway, rename might still work
			}
		}

		err := os.Rename(srcPath, dstPath)
		if err == nil {
			if attempt > 0 {
				klog.V(1).Infof("Windows file replacement succeeded after %d retries", attempt+1)
			}
			return nil // Success!
		}

		// Check if it's a Windows file locking error
		if isWindowsFileLockError(err) {
			if attempt < maxRetries-1 {
				// Exponential backoff: 50ms, 100ms, 200ms, 400ms, 800ms
				delay := baseDelay * time.Duration(1<<attempt)
				klog.V(2).Infof("Windows file lock detected (attempt %d/%d): %v - retrying in %v", 
					attempt+1, maxRetries, err, delay)
				time.Sleep(delay)
				continue
			} else {
				// Max retries reached with file lock error
				return errors.Wrap(err, "file temporarily locked (antivirus/Windows Defender scanning) - max retries exceeded")
			}
		}

		// Non-retryable error
		klog.V(2).Infof("Non-retryable error on Windows: %v", err)
		return err
	}

	return errors.New("file replacement failed after maximum retries")
}

// isWindowsFileLockError detects Windows-specific file locking errors that can be retried
func isWindowsFileLockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "access is denied") ||
		strings.Contains(errStr, "being used by another process") ||
		strings.Contains(errStr, "sharing violation") ||
		strings.Contains(errStr, "the process cannot access the file")
}

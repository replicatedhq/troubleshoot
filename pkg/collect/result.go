package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

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
	klog.V(2).Info("Creating symlink ", relativeLinkPath, " -> ", relativeFilePath)
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
	// lives in TarSupportBundleDir function. This path needs to
	// remain as-is to support memory only bundles e.g preflight
	err = os.Symlink(filePath, linkPath)
	if err != nil {
		return errors.Wrap(err, "failed to create symlink")
	}

	klog.V(2).Infof("Created %q symlink of %q", relativeLinkPath, relativeFilePath)
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

	tmpFile, err := os.CreateTemp("", "replace-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		return errors.Wrap(err, "failed to write tmp file")
	}

	// This rename should always be in /tmp, so no cross-partition copying will happen
	err = os.Rename(tmpFile.Name(), filepath.Join(bundlePath, relativePath))
	if err != nil {
		return errors.Wrap(err, "failed to rename tmp file")
	}

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

func TarSupportBundleDir(bundlePath string, input CollectorResult, outputFilename string) error {
	fileWriter, err := os.Create(outputFilename)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer fileWriter.Close()

	gzipWriter := gzip.NewWriter(fileWriter)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for relativeName := range input {
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

			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

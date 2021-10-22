package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type CollectorResult map[string][]byte

func NewResult() CollectorResult {
	return map[string][]byte{}
}

func (r CollectorResult) SaveResult(bundlePath string, relativePath string, reader io.Reader) error {
	if reader == nil {
		return nil
	}

	if bundlePath == "" {
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return errors.Wrap(err, "failed to read data")
		}
		r[relativePath] = data
		return nil
	}

	r[relativePath] = nil // save the file name referencing the file on disk

	fileDir, fileName := filepath.Split(relativePath)
	outPath := filepath.Join(bundlePath, fileDir)

	if err := os.MkdirAll(outPath, 0777); err != nil {
		return errors.Wrap(err, "create output file")
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
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return errors.Wrap(err, "failed to read data")
		}
		r[relativePath] = data
		return nil
	}

	tmpFile, err := ioutil.TempFile("", "replace-")
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

func (r CollectorResult) GetReader(bundlePath string, relativePath string) (io.Reader, error) {
	if r[relativePath] != nil {
		return bytes.NewReader(r[relativePath]), nil
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
		var b bytes.Buffer
		return &b, nil
	}

	fileDir, _ := filepath.Split(relativePath)
	outPath := filepath.Join(bundlePath, fileDir)
	if err := os.MkdirAll(outPath, 0777); err != nil {
		return nil, errors.Wrap(err, "create output file")
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

	if b, ok := writer.(*bytes.Buffer); ok {
		r[relativePath] = b.Bytes()
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
		info, err := os.Stat(filename)
		if err != nil {
			return errors.Wrap(err, "failed to stat file")
		}

		fileMode := info.Mode()
		if !fileMode.IsRegular() { // support bundle can have only files
			continue
		}

		parentDirName := filepath.Dir(bundlePath) // this is to have the files inside a subdirectory
		nameInArchive, err := filepath.Rel(parentDirName, filename)
		if err != nil {
			return errors.Wrap(err, "failed to create relative file name")
		}

		// tar.FileInfoHeader call causes a crash in static builds
		// https://github.com/golang/go/issues/24787
		hdr := &tar.Header{
			Name:     nameInArchive,
			ModTime:  info.ModTime(),
			Mode:     int64(fileMode.Perm()),
			Typeflag: tar.TypeReg,
			Size:     info.Size(),
		}

		err = tarWriter.WriteHeader(hdr)
		if err != nil {
			return errors.Wrap(err, "failed to write tar header")
		}

		err = func() error {
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

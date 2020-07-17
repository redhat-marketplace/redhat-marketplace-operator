package reporter

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
)

func TargzFolder(srcFolder, destFileName string) error {
	f, err := os.Create(destFileName)

	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}

	defer f.Close()

	return Tar(srcFolder, f)
}

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func Tar(src string, writers ...io.Writer) error {
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, errIn error) error {
		// return on any error
		if errIn != nil {
			return errors.Wrap(errIn, "fail to tar files")
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return errors.Wrap(err, "failed to create new dir")
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.ReplaceAll(file, src, ""), string(filepath.Separator))

		// write the header
		if err = tw.WriteHeader(header); err != nil {
			return errors.Wrap(err, "failed to write header")
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return errors.Wrap(err, "fail to open file for taring")
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return errors.Wrap(err, "failed to copy data")
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
}

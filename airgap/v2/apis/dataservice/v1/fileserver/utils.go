package fileserver

import (
	"crypto/sha256"
	"fmt"
	"io"

	"emperror.dev/errors"
)

func FileDownload_ProcessDownloadStream(w io.Writer, recv func() (*DownloadFileResponse, error)) (sha256Checksum string, err error) {
	var resp *DownloadFileResponse
	h := sha256.New()

	for {
		resp, err = recv()

		if err == io.EOF {
			break
		}

		if err != nil {
			err = fmt.Errorf("error while reading stream: %v", err)
			return
		}

		if data := resp.GetChunkData(); data != nil {
			_, err = w.Write(data)

			if err != nil {
				err = errors.Wrap(err, "failed to write file")
				return
			}

			_, err = h.Write(data)

			if err != nil {
				err = errors.Wrap(err, "failed to write file checksum")
				return
			}
		}
	}

	sha256Checksum = fmt.Sprintf("%x", h.Sum(nil))
	return
}

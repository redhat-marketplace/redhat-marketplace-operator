// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

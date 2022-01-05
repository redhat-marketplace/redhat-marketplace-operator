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

package uploaders

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type LocalFilePathUploader struct {
	LocalFilePath string
}

func (r *LocalFilePathUploader) UploadFile(ctx context.Context, file string, reader io.Reader) (string, error) {
	if _, err := os.Stat(r.LocalFilePath); err != nil {
		return "", err
	}

	if r.LocalFilePath == "" {
		r.LocalFilePath = "."
	}

	log := logger.WithValues("uploader", "localFilePath")

	baseName := filepath.Base(file)
	fileName := filepath.Join(r.LocalFilePath, baseName)

	log.Info("creating file", "name", fileName)
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Error(err, "Error creating", "name", fileName)
		return "", err
	}

	_, err = io.Copy(f, reader)
	if err != nil {
		log.Error(err, "Error opening input", "name", file)
		return "", err
	}

	return fileName, nil
}

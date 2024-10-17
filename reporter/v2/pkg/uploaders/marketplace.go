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
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
)

type MarketplaceUploaderConfig struct {
	URL                      string   `json:"url"`
	AuthorizeAccountCreation bool     `json:"authorizeAccountCreation"`
	Token                    string   `json:"-"`
	AdditionalCertFiles      []string `json:"additionalCertFiles,omitempty"`

	certificates []*x509.Certificate `json:"-"`
	httpVersion  *int                `json:"-"`

	CipherSuites []uint16
	MinVersion   uint16
}

type MarketplaceUploader struct {
	MarketplaceUploaderConfig
	client *http.Client
}

var _ Uploader = &MarketplaceUploader{}

func NewMarketplaceUploader(
	config *MarketplaceUploaderConfig,
) (Uploader, error) {
	tlsConfig, err := GenerateCACertPool(config.certificates, config.AdditionalCertFiles)

	if err != nil {
		return nil, err
	}

	tlsConfig.CipherSuites = config.CipherSuites
	tlsConfig.MinVersion = config.MinVersion

	client := &http.Client{}
	config.httpVersion = ptr.Int(1)

	// default to 2 unless otherwise overridden
	if config.httpVersion == nil {
		config.httpVersion = ptr.Int(2)
	}

	proxyCfg := httpproxy.FromEnvironment()
	if proxyCfg.HTTPProxy != "" || proxyCfg.HTTPSProxy != "" {
		config.httpVersion = ptr.Int(1)
	}

	// Use the proper transport in the client
	switch *config.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		}
	case 2:
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	if config.Token != "" {
		client.Transport = WithBearerAuth(client.Transport, config.Token)
	}

	return &MarketplaceUploader{
		client:                    client,
		MarketplaceUploaderConfig: *config,
	}, nil
}

type MarketplaceUsageResponseDetails struct {
	Code       string `json:"code"`
	StatusCode int    `json:"statusCode"`
	Retryable  bool   `json:"retryable,omitempty"`
}

type MarketplaceUsageResponse struct {
	RequestID string      `json:"requestId,omitempty"`
	Status    MktplStatus `json:"status,omitempty"`
	Message   string      `json:"message,omitempty"`
	ErrorCode string      `json:"errorCode,omitempty"`

	Details *MarketplaceUsageResponseDetails `json:"details,omitempty"`
}

type MktplStatus = string

const VerificationError = errors.Sentinel("verification")

func checkError(resp *http.Response, status MarketplaceUsageResponse, message string) error {
	if resp.StatusCode < 300 && resp.StatusCode >= 200 {
		return nil
	}

	err := errors.NewWithDetails(message, "code", resp.StatusCode, "message", status.Message, "errorCode", status.ErrorCode)

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return errors.WrapWithDetails(VerificationError, "status", status.Message)
	}

	return err
}

// https://sandbox.marketplace.redhat.com/metering/api/v2/metrics'
func (r *MarketplaceUploader) uploadFileRequest(fileName string, reader io.Reader) (*http.Request, error) {
	marketplaceUploadURL, err := url.Parse(fmt.Sprintf("%s/metering/api/v2/metrics", r.URL))
	if err != nil {
		return nil, err
	}

	q := marketplaceUploadURL.Query()
	q.Set("authorizeAccountCreation", strconv.FormatBool(r.AuthorizeAccountCreation))
	marketplaceUploadURL.RawQuery = q.Encode()

	var req *http.Request

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBase := filepath.Base(fileName)
	fileExt := filepath.Ext(fileBase)
	fileWithoutExt := fileBase[:len(fileBase)-len(fileExt)]

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileWithoutExt, fileBase))
	h.Set("Content-Type", "application/gzip")

	part, err := writer.CreatePart(h)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, reader)
	if err != nil {
		return nil, err
	}

	err = writer.Close()

	if err != nil {
		return nil, err
	}

	req, err = http.NewRequest("POST", marketplaceUploadURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, err
}

func (r *MarketplaceUploader) uploadFile(req *http.Request) (string, error) {
	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
		return "", errors.Wrap(err, "failed to post")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	logger.Info(
		"retrieved response",
		"statusCode", resp.StatusCode,
		"proto", resp.Proto,
		"body", string(body),
		"headers", resp.Header)

	status := MarketplaceUsageResponse{}
	jsonErr := json.Unmarshal(body, &status)

	if err := checkError(resp, status, "failed to upload"); err != nil {
		return status.RequestID, err
	}

	if jsonErr != nil {
		return "", err
	}

	return status.RequestID, err
}

func (r *MarketplaceUploader) UploadFile(ctx context.Context, fileName string, reader io.Reader) (id string, err error) {
	var req *http.Request

	req, err = r.uploadFileRequest(fileName, reader)

	if err != nil {
		return "", err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		id, err = r.uploadFile(req)

		if err != nil {
			if errors.Is(err, VerificationError) {
				err = nil
			} else {
				errors.Wrap(err, "file upload failed")
			}
			return
		}
	}()

	<-done
	return
}

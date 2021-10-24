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
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"time"

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type MarketplaceUploaderConfig struct {
	URL                 string   `json:"url"`
	Token               string   `json:"-"`
	AdditionalCertFiles []string `json:"additionalCertFiles,omitempty"`

	certificates []*x509.Certificate `json:"-"`
	httpVersion  *int                `json:"-"`
	polling      time.Duration       `json:"-"`
	timeout      time.Duration       `json:"-"`
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

	// default timeout 2 minutes
	if config.timeout == 0 {
		config.timeout = 2 * time.Minute
	}
	// default polling 15 seconds
	if config.polling == 0 {
		config.polling = 15 * time.Second
	}

	return &MarketplaceUploader{
		client:                    client,
		MarketplaceUploaderConfig: *config,
	}, nil
}

type MarketplaceUsageResponse struct {
	RequestID string      `json:"requestId,omitempty"`
	Status    MktplStatus `json:"status,omitempty"`
	Message   string      `json:"message,omitempty"`
	ErrorCode string      `json:"errorCode,omitempty"`
}

type MktplStatus = string

const (
	MktplStatusSuccess    MktplStatus = "success"
	MktplStatusInProgress MktplStatus = "inProgress"
	MktplStatusFailed     MktplStatus = "failed"
)

// https://sandbox.marketplace.redhat.com/usage/api/v2/metrics/<request id>
func (r *MarketplaceUploader) statusRequest(id string) (*MarketplaceUsageResponse, error) {
	marketplaceUploadURLCheck := "%s/usage/api/v2/metrics/%s"

	resp, err := r.client.Get(fmt.Sprintf(marketplaceUploadURLCheck, r.URL, id))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	status := MarketplaceUsageResponse{}
	jsonErr := json.Unmarshal(data, &status)

	if err := checkError(*resp, status, "failed to get status"); err != nil {
		return nil, err
	}

	if jsonErr != nil {
		return nil, err
	}

	return &status, nil
}

const RetryableError = errors.Sentinel("retryable")

func isRetryable(err error) bool {
	if errors.Is(err, RetryableError) {
		return true
	}
	return false
}

func checkError(resp http.Response, status MarketplaceUsageResponse, message string) error {
	if resp.StatusCode < 300 && resp.StatusCode >= 200 {
		return nil
	}

	err := errors.NewWithDetails(message, "code", resp.StatusCode, "message", status.Message, "errorCode", status.ErrorCode)

	if resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusTooEarly ||
		resp.StatusCode == http.StatusRequestTimeout {

		return errors.WrapWithDetails(RetryableError, "retryable error", append(errors.GetDetails(err), "message", err.Error())...)
	}

	return err
}

// https://sandbox.marketplace.redhat.com/usage/api/v2/metrics'
func (r *MarketplaceUploader) uploadFileRequest(fileName string, reader io.Reader) (*http.Request, error) {
	marketplaceUploadURL := "%s/usage/api/v2/metrics"

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

	req, err = http.NewRequest("POST", fmt.Sprintf(marketplaceUploadURL, r.URL), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, nil
}

func (r *MarketplaceUploader) uploadFile(req *http.Request) (string, error) {
	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
		return "", errors.Wrap(err, "failed to post")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

	if err := checkError(*resp, status, "failed to upload"); err != nil {
		return "", err
	}

	if jsonErr != nil {
		return "", err
	}

	return status.RequestID, err
}

var DefaultBackoff = wait.Backoff{
	Steps:    4,
	Duration: 50 * time.Millisecond,
	Factor:   5.0,
	Jitter:   0.1,
}

func (r *MarketplaceUploader) UploadFile(ctx context.Context, fileName string, reader io.Reader) (id string, err error) {
	var req *http.Request

	req, err = r.uploadFileRequest(fileName, reader)

	if err != nil {
		return "", err
	}

	done := make(chan struct{})

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	ticker := time.NewTicker(r.polling)
	defer ticker.Stop()

	go func() {
		defer close(done)

		err = retry.OnError(DefaultBackoff, isRetryable, func() error {
			localId, err := r.uploadFile(req)

			if err != nil {
				return errors.Wrap(err, "failed to get upload file req")
			}

			id = localId
			return nil
		})

		if err != nil {
			return
		}

		for {
			var resp MarketplaceUsageResponse
			err = retry.OnError(DefaultBackoff, isRetryable, func() error {
				var err error
				innerResp, err := r.statusRequest(id)

				if err != nil {
					return err
				}

				if innerResp == nil {
					return errors.New("no response provided")
				}

				resp = *innerResp
				return nil
			})

			if err != nil {
				logger.Error(err, "failed to get status")
			}

			if resp.Status == MktplStatusSuccess {
				return
			}
			if resp.Status == MktplStatusFailed {
				err = errors.NewWithDetails("upload processing failed", "message", resp.Message, "code", resp.ErrorCode)
				return
			}

			select {
			case <-ctx.Done():
				err = errors.New("couldn't verify upload, context was cancelled")
				return
			case <-ticker.C:
			}
		}
	}()

	<-done
	return
}

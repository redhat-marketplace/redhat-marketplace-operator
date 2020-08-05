package reporter

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	"golang.org/x/net/http2"
)

type RedHatInsightsUploaderConfig struct {
	URL                 string   `json:"url"`
	Token               string   `json:"-"`
	OperatorVersion     string   `json:"operatorVersion"`
	ClusterID           string   `json:"clusterID"`
	AdditionalCertFiles []string `json:"additionalCertFiles,omitempty"`
	httpVersion         *int     `json:"-"`
}

type RedHatInsightsUploader struct {
	RedHatInsightsUploaderConfig
	client *http.Client
}

func NewRedHatInsightsUploader(
	config *RedHatInsightsUploaderConfig,
) (*RedHatInsightsUploader, error) {
	tlsConfig, err := generateCACertPool(config.AdditionalCertFiles...)

	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	// default to 2 unless otherwise overridden
	if config.httpVersion == nil {
		config.httpVersion = ptr.Int(2)
	}

	// Use the proper transport in the client
	switch *config.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	case 2:
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return &RedHatInsightsUploader{
		client:                       client,
		RedHatInsightsUploaderConfig: *config,
	}, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

const mktplaceFileUploadType = "application/vnd.redhat.mkt.tar+tgz"
const userAgentFmt = "marketplace-operator/%s cluster/%s"

const uploadURL = "%s/api/ingress/v1/upload"

func getUserAgent(version, clusterID string) string {
	return fmt.Sprintf(userAgentFmt, version, clusterID)
}

func (r *RedHatInsightsUploader) uploadFileRequest(path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes("file"), escapeQuotes(filepath.Base(path))))
	h.Set("Content-Type", mktplaceFileUploadType)
	part, err := writer.CreatePart(h)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)

	if err != nil {
		return nil, err
	}

	_ = writer.WriteField("type", mktplaceFileUploadType)

	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf(uploadURL, r.URL), body)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))
	req.Header.Set("User-Agent", getUserAgent(r.OperatorVersion, r.ClusterID))
	return req, err
}

func (r *RedHatInsightsUploader) UploadFile(path string) error {
	req, err := r.uploadFileRequest(path)

	if err != nil {
		return errors.Wrap(err, "failed to get upload file req")
	}

	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
		return errors.Wrap(err, "failed to post")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	logger.Info(
		"retrieved response",
		"statusCode", resp.StatusCode,
		"proto", resp.Proto,
		"body", string(body),
		"headers", resp.Header)

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return errors.NewWithDetails("failed to upload field",
			"statusCode", resp.StatusCode,
			"proto", resp.Proto,
			"body", string(body),
			"headers", resp.Header)
	}
	return nil
}

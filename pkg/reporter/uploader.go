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
	"golang.org/x/net/http2"
)

type RedHatInsightsUploader struct {
	Url             string
	Token           string
	OperatorVersion string
	ClusterID       string
	httpVersion     int
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

const contentType = "application/vnd.redhat.hccm.tar+tgz"
const userAgentFmt = "marketplace-operator/%s cluster/%s"

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
	h.Set("Content-Type", contentType)
	part, err := writer.CreatePart(h)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)

	if err != nil {
		return nil, err
	}

	_ = writer.WriteField("type", contentType)

	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", r.Url, body)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))
	req.Header.Set("User-Agent", getUserAgent(r.OperatorVersion, r.ClusterID))
	return req, err
}

func (r *RedHatInsightsUploader) UploadFile(path string) error {
	client, err := r.getClient()

	if err != nil {
		return errors.Wrap(err, "failed to get client")
	}

	req, err := r.uploadFileRequest(path)

	if err != nil {
		return errors.Wrap(err, "failed to get upload file req")
	}

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err, "failed to post")
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
		"body", string(body))

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return errors.NewWithDetails("failed to upload field",
			"statusCode", resp.StatusCode,
			"proto", resp.Proto,
			"body", string(body))
	}
	return nil
}

func (r *RedHatInsightsUploader) getClient() (*http.Client, error) {
	client := &http.Client{}

	// Create a pool with the server certificate since it is not signed
	// by a known CA
	// caCert, err := ioutil.ReadFile("server.crt")
	// if err != nil {
	// 	log.Fatalf("Reading server certificate: %s", err)
	// }
	// caCertPool := x509.NewCertPool()
	// caCertPool.AppendCertsFromPEM(caCert)

	// Create TLS configuration with the certificate of the server
	// tlsConfig := &tls.Config{
	// 	RootCAs: caCertPool,
	// }

	// Use the proper transport in the client
	switch r.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			//TLSClientConfig: tlsConfig,
		}
	case 2:
		client.Transport = &http2.Transport{
			//TLSClientConfig: tlsConfig,
		}
	}

	return client, nil

}

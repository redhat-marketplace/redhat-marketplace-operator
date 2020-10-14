package connect

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"emperror.dev/errors"
)

// client
const domain = "https://connect.redhat.com/api/v2"

type connectClient struct {
	*http.Client
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithHeader(rt http.RoundTripper) withHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return withHeader{Header: make(http.Header), rt: rt}
}

func (h withHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.Header {
		req.Header[k] = v
	}

	return h.rt.RoundTrip(req)
}

func NewConnectClient(token string) *connectClient {
	client := http.DefaultClient
	rt := WithHeader(client.Transport)
	rt.Add("Content-Type", "application/json")
	rt.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	client.Transport = rt

	return &connectClient{
		Client: client,
	}
}

type scanResults struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Code    int32    `json:"code"`
	Data    scanData `json:"data"`
}

type scanData struct {
	ScanProfile string          `json:"scan_profile"`
	Results     map[string]bool `json:"results"`
}

func (s *scanResults) IsPassed() bool {
	for _, result := range s.Data.Results {
		if !result {
			return false
		}
	}

	return true
}

type connectResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    int32  `json:"code"`
}

func (c *connectResponse) IsOK() bool {
	return c.Code == 200
}

func (c *connectResponse) IsPublished() bool {
	return c.Code == 423
}

func (c *connectClient) GetDigestStatus(opsid, digest string) (*connectResponse, *scanResults, error) {
	url := fmt.Sprintf("%s/container/%s/certResults/%s", domain, opsid, digest)
	fmt.Printf("url is %s\n", url)
	resp, err := c.Get(url)
	defer resp.Body.Close()

	if err != nil {
		return nil, nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, nil, errors.Wrap(err, "error reading body")
	}

	var cResp connectResponse
	err = json.Unmarshal(body, &cResp)

	fmt.Printf("scanResults %s\n", string(body))

	if err != nil {
		return nil, nil, errors.Wrap(err, "error json marshalling")
	}

	var rawJson map[string]interface{}
	err = json.Unmarshal(body, &rawJson)

	if err != nil {
		return &cResp, nil, errors.Wrap(err, "error json marshalling")
	}

	if !cResp.IsOK() {
		return &cResp, nil, nil
	}

	if _, ok := rawJson["data"].([]interface{}); ok {
		return &cResp, nil, nil
	}

	var data scanResults
	err = json.Unmarshal(body, &data)

	if err != nil {
		return &cResp, &scanResults{
			Code:    cResp.Code,
			Status:  cResp.Status,
			Message: cResp.Message,
		}, nil
	}

	return &cResp, &data, nil
}

func (c *connectClient) PublishDigest(opsid, digest, tag string) (*connectResponse, error) {
	url := fmt.Sprintf("%s/projects/%s/containers/%s/tags/%s/publish", domain, opsid, digest, tag)
	resp, err := c.Post(url, "application/json", strings.NewReader("{}"))
	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, errors.Wrap(err, "error reading body")
	}

	var data connectResponse
	err = json.Unmarshal(body, &data)

	if err != nil {
		fmt.Println(string(body))
		return nil, errors.Wrap(err, "error json marshalling")
	}

	return &data, nil
}

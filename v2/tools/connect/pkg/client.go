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

package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Code    int32               `json:"code"`
	Data    connectResponseData `json:"data,omitempty"`
}

type connectResponseData struct {
	Errors []string `json:"errors,omitempty"`
}

func (c *connectResponse) IsOK() bool {
	return c.Code == 200
}

func (c *connectResponse) IsError() bool {
	return c.Code >= 300
}

func (c *connectResponse) IsAlreadyPublished() bool {
	return c.Code == 423 &&
		len(c.Data.Errors) == 1 &&
		strings.Contains(c.Data.Errors[0], "Container image is already published")
}

type tagResponse struct {
	Tags []pcTag `json:"tags"`
}

type pcTag struct {
	Digest      string
	Name        string
	HealthIndex string `json:"health_index"`
	Published   bool
	ScanStatus  string `json:"scan_status"`
}

func (p *pcTag) String() string {
	return p.Digest + " " + p.Name + " " + p.ScanStatus
}

func (c *connectClient) PublishDigest(opsid, digest, tag string) (*connectResponse, error) {
	projectURL := fmt.Sprintf("%s/projects/%s/containers/%s/tags/%s/publish?latest=true", domain, opsid, digest, tag)
	u, _ := url.Parse(projectURL)

	fmt.Println(u.String())

	resp, err := c.Post(u.String(), "application/json", strings.NewReader("{}"))
	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)

	fmt.Printf("publishResults %s\n", string(body))

	if err != nil {
		return nil, errors.Wrap(err, "error reading body")
	}

	var data connectResponse
	err = json.Unmarshal(body, &data)

	if err != nil {
		return nil, errors.Wrap(err, "error json marshalling")
	}

	return &data, nil
}

func (c *connectClient) GetTag(opsid, digest string) (*pcTag, error) {
	projectURL := fmt.Sprintf("%s/projects/%s/tags", domain, opsid)
	req, err := http.NewRequest("POST", projectURL, strings.NewReader("{}"))
	//base, err := url.Parse(projectURL)

	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("digests", digest)
	req.URL.RawQuery = q.Encode()

	resp, err := c.Do(req)
	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("non 200 response %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Printf("body: %s\n", string(body))

	if err != nil {
		return nil, errors.Wrap(err, "error reading body")
	}

	var data tagResponse
	err = json.Unmarshal(body, &data)

	if err != nil {
		return nil, errors.Wrap(err, "error json marshalling")
	}

	if len(data.Tags) == 0 {
		return nil, nil
	}

	return &data.Tags[0], nil
}

// Copyright 2024 IBM Corp.
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

/*
  - altDestinations:
    - confirmDelivery: false
      headers:
        secretRef: header-map-secret
      insecureSkipTLSVerify: false
      transformer:
        configMapKeyRef:
          key: kazaam.json
          name: kazaam-configmap
        type: kazaam
      url: https://cmmEndpoint.x.y.z:1234/api/stuff
*/

package uploader

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"sync"

	"emperror.dev/errors"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

const (
	Non200Response = "non 200 response"
)

type Uploader struct {
	client *http.Client

	config *Config

	// derived config values
	destURL           *url.URL
	destURLSuffixExpr *jp.Expr
	authURL           *url.URL
	authTokenExpr     *jp.Expr
	destHeader        http.Header
	authHeader        http.Header
	authBodyData      []byte

	// multiple threads may attempt to refresh token
	mu        sync.RWMutex
	authToken string
}

// Uploader config for construction
type Config struct {
	DestURL              string
	DestHeader           map[string]string
	DestURLSuffixExpr    string
	AuthURL              string
	AuthHeader           map[string]string
	AuthDestHeader       string
	AuthDestHeaderPrefix string
	AuthTokenExpr        string
	AuthBodyData         []byte
}

// Optional: Defines an endpoint to call to request an authorization token used when making the destURL request
// authURL: authorizataion endpoint
// authHeader: headers sent when sending the authorization request
// authDestHeader: the additional header map key to set on the destHeader ("Authorization")
// authDestPrefix: the additional prefix map string value to set on the destHeader ("Bearer ")
// parseResponse: optionally jsonpath parse the response for the authorization token
func NewUploader(client *http.Client, config *Config) (u *Uploader, err error) {

	u = &Uploader{
		client: client,
		config: config,
	}

	// Configure Destination

	u.destURL, err = url.Parse(config.DestURL)
	if err != nil {
		return
	}

	u.destHeader = make(http.Header)
	for k, v := range config.DestHeader {
		u.destHeader.Set(k, v)
	}

	if len(config.DestURLSuffixExpr) != 0 {
		expr, err := jp.ParseString(config.DestURLSuffixExpr)
		u.destURLSuffixExpr = &expr
		if err != nil {
			return nil, err
		}
	}

	// Configuration Authorization

	if len(config.AuthURL) != 0 {
		u.authURL, err = url.Parse(config.AuthURL)
		if err != nil {
			return
		}
	}

	u.authHeader = make(http.Header)
	for k, v := range config.AuthHeader {
		u.authHeader.Add(k, v)
	}

	if len(config.AuthDestHeader) != 0 {
		expr, err := jp.ParseString(config.AuthTokenExpr)
		u.authTokenExpr = &expr
		if err != nil {
			return nil, err
		}
	}

	u.authBodyData = config.AuthBodyData

	return
}

func (u *Uploader) TransformAndUpload(eventMsg []byte) (int, error) {

	// Parse for the optional URL suffix
	destURL, err := url.Parse(u.destURL.String())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	destURL = u.destURL.JoinPath(u.parseForSuffix(eventMsg))

	// TODO: Transform

	// Upload
	return u.upload(destURL, eventMsg)
}

func (u *Uploader) upload(destURL *url.URL, body []byte) (int, error) {

	dResp, err := u.uploadToDest(destURL, body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	defer dResp.Body.Close()

	if dResp.StatusCode == http.StatusForbidden && u.authURL != nil {
		// Request was not authorized, attempt to request a authorization token
		statusCode, err := u.callAuth()
		if err != nil {
			return statusCode, err
		}

		// With new token, try destination again
		adResp, err := u.uploadToDest(destURL, body)
		if err != nil {
			return adResp.StatusCode, err
		}

		defer adResp.Body.Close()

		if adResp.StatusCode != http.StatusOK {
			return adResp.StatusCode, errors.NewWithDetails(Non200Response, "url", u.destURL.String(), "statuscode", adResp.StatusCode)
		}

	} else if dResp.StatusCode != http.StatusOK {
		return dResp.StatusCode, errors.NewWithDetails(Non200Response, "url", u.destURL.String(), "statuscode", dResp.StatusCode)
	}

	return dResp.StatusCode, nil
}

func (u *Uploader) uploadToDest(destURL *url.URL, body []byte) (*http.Response, error) {

	dReq, err := http.NewRequest(http.MethodPost, destURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	dReq.Header = u.destHeader
	if len(u.config.AuthDestHeader) != 0 && len(u.getAuthToken()) != 0 {
		dReq.Header.Set(u.config.AuthDestHeader, u.config.AuthDestHeaderPrefix+u.getAuthToken())
	}

	return u.client.Do(dReq)
}

func (u *Uploader) parseForSuffix(eventMsg []byte) (suffix string) {
	// Parse for the optional URL suffix
	if u.destURLSuffixExpr != nil {
		results := u.destURLSuffixExpr.Get(eventMsg)
		if len(results) != 0 {
			suffix = oj.JSON(results[0])
		}
	}
	return
}

func (u *Uploader) callAuth() (int, error) {

	aReq, err := http.NewRequest(http.MethodPost, u.authURL.String(), bytes.NewReader(u.authBodyData))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	aReq.Header = u.authHeader

	aResp, err := u.client.Do(aReq)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	defer aResp.Body.Close()

	if aResp.StatusCode != http.StatusOK {
		return aResp.StatusCode, errors.NewWithDetails(Non200Response, "url", u.authURL.String(), "statuscode", aResp.StatusCode)
	}

	aBody, err := io.ReadAll(aResp.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// If an expression is configured, parse the body for the first result as token
	if u.authTokenExpr != nil {
		results := u.authTokenExpr.Get(aBody)
		if len(results) != 0 {
			u.setAuthToken(oj.JSON(results[0]))
		}
	} else {
		u.setAuthToken(string(aBody))
	}

	return aResp.StatusCode, nil
}

func (u *Uploader) getAuthToken() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.authToken
}

func (u *Uploader) setAuthToken(token string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.authToken = token
}

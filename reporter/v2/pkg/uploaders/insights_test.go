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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("insights", func() {
	It("should ", func() {
		Expect(true).To(BeTrue())

	})
})

// uploader, err = uploaders.NewRedHatInsightsUploader(&uploaders.RedHatInsightsUploaderConfig{
// 	URL:             "https://cloud.redhat.com",
// 	ClusterID:       "2858312a-ff6a-41ae-b108-3ed7b12111ef",
// 	OperatorVersion: "1.0.0",
// 	Token:           "token",
// })
// Expect(err).To(Succeed())

// uploader.(*uploaders.RedHatInsightsUploader).client.Transport = &stubRoundTripper{
// 	roundTrip: func(req *http.Request) *http.Response {
// 		headers := make(http.Header)
// 		headers.Add("content-type", "text")

// 		Expect(req.URL.String()).To(Equal(fmt.Sprintf(uploadURL, uploader.(*uploaders.RedHatInsightsUploader).URL)), "url does not match expected")
// 		_, meta, _ := req.FormFile("file")
// 		header := meta.Header

// 		Expect(header.Get("Content-Type")).To(Equal(mktplaceFileUploadType))
// 		Expect(header.Get("Content-Disposition")).To(
// 			HavePrefix(`form-data; name="%s"; filename="%s"`, "file", "test-upload.tar.gz"))
// 		Expect(req.Header.Get("Content-Type")).To(HavePrefix("multipart/form-data; boundary="))
// 		Expect(req.Header.Get("User-Agent")).To(HavePrefix("marketplace-operator"))

// 		return &http.Response{
// 			StatusCode: 202,
// 			// Send response to be tested
// 			Body: ioutil.NopCloser(bytes.NewBuffer([]byte{})),
// 			// Must be set to non-nil value or it panics
// 			Header: headers,
// 		}
// 	},
// }

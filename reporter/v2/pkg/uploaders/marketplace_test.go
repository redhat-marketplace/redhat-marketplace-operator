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
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("marketplace uploaders", func() {
	var (
		err    error
		server *ghttp.Server
		sut    Uploader

		postReponse               MarketplaceUsageResponse
		getResponse, getResponse2 MarketplaceUsageResponse
		retryPostResponse         MarketplaceUsageResponse

		fileName = "test"

		testId = "testId"

		testBody = []byte("foo")

		config *MarketplaceUploaderConfig
	)

	BeforeEach(func() {
		server = ghttp.NewTLSServer()
		config = &MarketplaceUploaderConfig{
			URL:     server.URL(),
			Token:   "foo",
			polling: 1 * time.Second,
			timeout: 4 * time.Second,
			certificates: []*x509.Certificate{
				server.HTTPTestServer.Certificate(),
			},
		}
		sut, err = NewMarketplaceUploader(config)
		Expect(err).To(Succeed())

		postReponse = MarketplaceUsageResponse{RequestID: testId}
		retryPostResponse = MarketplaceUsageResponse{
			Details: &MarketplaceUsageResponseDetails{
				Code:      "409",
				Retryable: true,
			},
		}
		getResponse = MarketplaceUsageResponse{
			Status: MktplStatusInProgress,
		}
		getResponse2 = MarketplaceUsageResponse{
			Status: MktplStatusSuccess,
		}
	})

	AfterEach(func() {
		server.Close()
	})
	Describe("uploading files", func() {
		BeforeEach(func() {
			testId = "6bf1a9e41041d7d6913bbbcbc23c2a137ee170bbe7ebf58cf886fdb9c66989ff"
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWith(http.StatusAccepted, "{\"requestId\":\"6bf1a9e41041d7d6913bbbcbc23c2a137ee170bbe7ebf58cf886fdb9c66989ff\"}"),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse2),
				),
			)
		})

		It("should upload a file", func() {
			ctx := context.Background()
			id, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).ToNot(HaveOccurred())
			Expect(id).To(Equal(testId))
		})
	})

	Describe("uploading files", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					ghttp.RespondWithJSONEncoded(http.StatusTooManyRequests, &retryPostResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWithJSONEncoded(http.StatusAccepted, &postReponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusTooManyRequests, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse2),
				),
			)
		})

		It("should upload a file and retry", func() {
			ctx := context.Background()
			id, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).ToNot(HaveOccurred())
			Expect(id).To(Equal(testId))
		})
	})

	Describe("uploading files", func() {

		BeforeEach(func() {
			getResponse = MarketplaceUsageResponse{
				Status:    MktplStatusFailed,
				Message:   "failed",
				ErrorCode: "100",
			}

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWithJSONEncoded(http.StatusAccepted, &postReponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
			)
		})

		It("should return error on failure", func() {
			ctx := context.Background()
			_, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("uploading files", func() {
		BeforeEach(func() {
			config.polling = 1
			config.timeout = 1
			sut, err = NewMarketplaceUploader(config)
			Expect(err).To(Succeed())

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWithJSONEncoded(http.StatusAccepted, &postReponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/metering/api/v2/metrics/"+testId),
					ghttp.RespondWithJSONEncoded(http.StatusOK, &getResponse),
				),
			)
		})

		It("should timeout", func() {
			ctx := context.Background()
			_, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("handling conflict", func() {
		BeforeEach(func() {
			sut, err = NewMarketplaceUploader(config)
			Expect(err).To(Succeed())

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWith(http.StatusConflict, `{"errorCode":"document_conflict","message":"Upload is duplicate of previous submission","details":{"code":"document_conflict","statusCode":409,"retryable":false}}`),
				),
			)
		})

		It("should handle duplicate conflict", func() {
			ctx := context.Background()
			id, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).ToNot(HaveOccurred())
			Expect(id).To(BeEmpty())
		})
	})

	Describe("handling verification error", func() {
		BeforeEach(func() {
			sut, err = NewMarketplaceUploader(config)
			Expect(err).To(Succeed())

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
					verifyFileUpload(fileName, testBody),
					ghttp.RespondWith(http.StatusUnprocessableEntity, `{"errorCode":"document_conflict","requestID":"foo","message":"Verification errors","details":{"code":"document_conflict","statusCode":409,"retryable":false}}`),
				),
			)
		})

		It("should handle duplicate conflict", func() {
			ctx := context.Background()
			id, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).ToNot(HaveOccurred())
			Expect(id).To(Equal("foo"))
		})
	})

	Describe("handling error", func() {
		BeforeEach(func() {
			sut, err = NewMarketplaceUploader(config)
			Expect(err).To(Succeed())

			for i := 0; i < 4; i++ {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/metering/api/v2/metrics"),
						verifyFileUpload(fileName, testBody),
						ghttp.RespondWith(http.StatusInternalServerError, `{"errorCode":"internal_application_error_ocurred","message":"Save usage result not ok or missing value, result {\"lastErrorObject\":{\"n\":0,\"updatedExisting\":false},\"value\":null,\"ok\":1,\"$clusterTime\":{\"clusterTime\":{\"$timestamp\":\"7025668740716953620\"},\"signature\":{\"hash\":\"H2Zbl5S64rst/CWWEsRupwqyUZs=\",\"keyId\":{\"low\":2,\"high\":1628832003,\"unsigned\":false}}},\"operationTime\":{\"$timestamp\":\"7025668740716953620\"}}, Retry UsageStatus.save failed after retry attempts: 3 duration: 3069 ms","details":{"code":"internal_application_error_ocurred","statusCode":500,"retryable":true}}`),
					),
				)
			}
		})

		It("should handle error", func() {
			ctx := context.Background()
			_, err := sut.UploadFile(ctx, fileName, bytes.NewReader(testBody))
			Expect(err).To(HaveOccurred())
		})
	})
})

func verifyFileUpload(fileName string, testBody []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		Expect(req.Header.Get("Content-Type")).To(ContainSubstring("multipart/form-data"))

		err := req.ParseMultipartForm(32 << 20) // maxMemory 32 MB
		Expect(err).To(Succeed())

		file, _, err := req.FormFile(fileName)
		Expect(err).To(Succeed())

		buff := &bytes.Buffer{}
		io.Copy(buff, file)

		Expect(buff.Bytes()).To(Equal(testBody))
		Expect(file.Close()).To(Succeed())
	}
}

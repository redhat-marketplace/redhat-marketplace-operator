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

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/datafilter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
)

var (
	testEnv        *envtest.Environment
	cfg            *rest.Config
	log            logr.Logger
	k8sClient      client.Client
	k8sManager     ctrl.Manager
	httpTestClient *http.Client
	httpTestServer *httptest.Server
	certDir        string
	grpcListen     net.Listener
	grpcServer     *grpc.Server
	dataFilters    *datafilter.DataFilters
	eventEngine    *events.EventEngine
	eventConfig    *events.Config
	cc             *v1alpha1.ComponentConfig
	eeCtx          context.Context
	eeCancel       context.CancelFunc
	metadataMap    map[string]string
)

const (
	usagePath = "/metering/api/1.0/usage"
	// productId should be parsed as the added suffix to the path called
	suffixUsagePath = "/metering/api/1.0/usage/1234"
	tokenPath       = "/api/2.0/accounts/1234/apikeys/token"

	testData = `{
		"anonymousId": "f5d26e39-d421-4367-b623-4c8a7bbf6cbf",
		"event": "Account Contractual Usage",
		"properties": {
			"accountId": "US_123456",
			"accountIdType": "countryCode_ICN",
			"accountPlan": "STL",
			"chargePlanType": 2,
			"daysToExpiration": 9999,
			"environment": "PRODUCTION",
			"eventId": "1234567890123",
			"frequency": "Hourly",
			"hyperscalerChannel": "ibm",
			"hyperscalerFormat": "saas",
			"hyperscalerProvider": "aws",
			"instanceGuid": "123456789012-testcorp",
			"instanceName": "",
			"productCode": "WW1234",
			"productCodeType": "WWPC",
			"productId": "1234-M12",
			"productTitle": "Test Application Suite",
			"productVersion": "1.23.4",
			"quantity": 123,
			"salesOrderNumber": "1234X",
			"source": "123456789abc",
			"subscriptionId": "1234",
			"tenantId": "1234",
			"unit": "Points",
			"unitDescription": "Points Description",
			"unitMetadata": {
				"data": {
					"assist-Users": 0,
					"core-Install": 100,
					"health-AuthorizedUsers": 0,
					"health-ConcurrentUsers": 0,
					"health-MAS-Internal-Scores": 0,
					"hputilities-MAS-AIO-Models": 0,
					"hputilities-MAS-AIO-Scores": 0,
					"hputilities-MAS-External-Scores": 0,
					"manage-Database-replicas": 0,
					"manage-Install": 150,
					"manage-MAS-Base": 0,
					"manage-MAS-Base-Authorized": 0,
					"manage-MAS-Limited": 0,
					"manage-MAS-Limited-Authorized": 0,
					"manage-MAS-Premium": 0,
					"manage-MAS-Premium-Authorized": 5,
					"monitor-IOPoints": 0,
					"monitor-KPIPoints": 0,
					"predict-MAS-Models-Trained": 0,
					"predict-MAS-Predictions-Count": 0,
					"visualinspection-MAS-Images-Inferred": 0,
					"visualinspection-MAS-Models-Trained": 0,
					"visualinspection-MAS-Videos-Inferred": 0
				},
				"version": "1"
			},
			"UT30": "12BH3"
		},
		"timestamp": "2024-01-18T11:00:11.159442+00:00",
		"type": "track",
		"userId": "ABCid-123456789abc-owner",
		"writeKey": "write-key"
	}`

	goodResult = `{"instances":[{"metricUsage":[{"quantity":123,"metricId":"Points"}],"instanceId":"123456789abc","startTime":"2024-01-18T11:00:11.159442+00:00","endTime":"2024-01-18T11:00:11.159442+00:00"}],"subscriptionId":"1234-M12"}`

	testDataBad = `{"event":"myevent"`

	myApiKey      = `{"apikey": "myapikey"}`
	myBearerToken = "Bearer mytoken"
	myTokenResp   = `{"token": "mytoken"}`
	appJsonHeader = "application/json"
)

var _ = BeforeSuite(func() {
	var err error

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	// Test HTTP Server & Client

	fmt.Println("httpTestServerdefine")
	logf.Log.Info("httpTestDefine")
	httpTestServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the token is not authorized, 401 and the datafilter should attempt the authorization endpoint
		// Check that the data arriving is the expected transformed data
		fmt.Println("httpTestServer handle request")
		switch r.URL.Path {

		// productId is parsed as teh
		case suffixUsagePath:
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				logf.Log.Error(err, "usage bad body")
				w.WriteHeader(http.StatusInternalServerError)
			}
			if r.Header.Get("Authorization") != myBearerToken {
				logf.Log.Info("httpTest", "usage authorization", http.StatusUnauthorized, "header", r.Header, "body", string(bodyBytes))
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				ok, err := checkJSONBytesEqual(bodyBytes, []byte(goodResult))
				if err != nil || !ok {
					logf.Log.Error(err, "usage bytes not equal", "header", r.Header, "body", string(bodyBytes))
					w.WriteHeader(http.StatusInternalServerError)
				}
				logf.Log.Info("httpTest", "usage status", http.StatusOK)
				w.WriteHeader(http.StatusOK)
			}

		case tokenPath:
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				logf.Log.Error(err, "token bad body")
				w.WriteHeader(http.StatusInternalServerError)
			}
			if (r.Header.Get("accept") != appJsonHeader) || (r.Header.Get("Content-Type") != appJsonHeader) {
				logf.Log.Info("httpTest", "token status", http.StatusBadRequest, "header", r.Header, "body", string(bodyBytes))
				w.WriteHeader(http.StatusBadRequest)
			} else {
				ok, err := checkJSONBytesEqual(bodyBytes, []byte(myApiKey))
				if err != nil || !ok {
					logf.Log.Error(err, "token apikey not correct", "header", r.Header, "body", string(bodyBytes))
					w.WriteHeader(http.StatusBadRequest)
				}
				logf.Log.Info("httpTest", "token status", http.StatusOK)
				w.Write([]byte(myTokenResp))
			}

		default:
			logf.Log.Info("httpTest", "default status", http.StatusNotFound, "path", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	logf.Log.Info("httpTest server started", "url", httpTestServer.URL)

	httpTestURL, err := url.Parse(httpTestServer.URL)
	Expect(err).ToNot(HaveOccurred())

	usageURL := httpTestURL.JoinPath(usagePath)
	tokenURL := httpTestURL.JoinPath(tokenPath)

	httpTestClient = httpTestServer.Client()

	// Temp dir for server cert

	certDir, err := os.MkdirTemp("", "cert")
	Expect(err).ToNot(HaveOccurred())

	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	genCert(certPath, keyPath)

	// Test grpc server (DataService)

	tlsCreds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
	Expect(err).ToNot(HaveOccurred())

	grpcListen, err = net.Listen("tcp", "localhost:8004")
	Expect(err).ToNot(HaveOccurred())

	grpcServer = grpc.NewServer(grpc.Creds(tlsCreds))
	reflection.Register(grpcServer)

	go func() {
		grpcServer.Serve(grpcListen)
	}()

	// Event Engine
	cc := v1alpha1.NewComponentConfig()

	tlsVersion, err := k8sapiflag.TLSVersion(cc.TLSConfig.MinVersion)
	Expect(err).ToNot(HaveOccurred())

	cipherSuites, err := k8sapiflag.TLSCipherSuites(cc.TLSConfig.CipherSuites)
	Expect(err).ToNot(HaveOccurred())

	dataServiceURL, err := url.Parse("localhost:8004")
	Expect(err).ToNot(HaveOccurred())

	eventConfig = &events.Config{
		LicenseAccept:        true,
		OutputDirectory:      os.TempDir(),
		DataServiceTokenFile: "",
		DataServiceCertFile:  certPath,
		DataServiceURL:       dataServiceURL,
		Namespace:            "default",
		AccMemoryLimit:       cc.AccMemoryLimit,
		MaxFlushTimeout:      cc.MaxFlushTimeout,
		MaxEventEntries:      cc.MaxEventEntries,
		CipherSuites:         cipherSuites,
		MinVersion:           tlsVersion,
	}

	eeCtx, eeCancel = context.WithCancel(context.Background())
	eventEngine = events.NewEventEngine(eeCtx, ctrl.Log, eventConfig, k8sClient)

	go eventEngine.Start(eeCtx)

	Eventually(func() bool {
		return eventEngine.IsReady()
	}).Should(BeTrue())

	// DataFilters
	dataFilters = datafilter.NewDataFilters(ctrl.Log.WithName("datafilter"),
		k8sClient, httpTestClient, eventEngine, eventConfig, &cc.ApiHandlerConfig)

	// k8s Configuration Objects
	metadataMap = make(map[string]string)
	metadataMap["k"] = "v"
	dataReporterConfig := &v1alpha1.DataReporterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "datareporterconfig",
			Namespace: "default",
		},
		Spec: v1alpha1.DataReporterConfigSpec{
			UserConfigs: []v1alpha1.UserConfig{
				v1alpha1.UserConfig{
					UserName: "testuser",
					Metadata: metadataMap,
				},
			},
			DataFilters: []v1alpha1.DataFilter{
				v1alpha1.DataFilter{
					Selector: v1alpha1.Selector{
						MatchExpressions: []string{
							`$.properties.productId`,
							`$[?($.properties.source != null)]`,
							`$[?($.properties.unit == "Points")]`,
							`$[?($.properties.quantity >= 0)]`,
							`$[?($.timestamp != null)]`,
						},
						MatchUsers: []string{"testuser"},
					},
					ManifestType: "dataReporter",
					Transformer: v1alpha1.Transformer{
						TransformerType: "kazaam",
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "kazaam-configmap",
							},
							Key: "kazaam.json",
						},
					},
					AltDestinations: []v1alpha1.Destination{
						v1alpha1.Destination{
							Transformer: v1alpha1.Transformer{
								TransformerType: "kazaam",
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kazaam-configmap",
									},
									Key: "kazaam.json",
								},
							},
							URL:           usageURL.String(),
							URLSuffixExpr: "$.properties.subscriptionId",
							Header: v1alpha1.Header{
								Secret: corev1.LocalObjectReference{
									Name: "dest-header-map-secret",
								},
							},
							Authorization: v1alpha1.Authorization{
								URL: tokenURL.String(),
								Header: v1alpha1.Header{
									Secret: corev1.LocalObjectReference{
										Name: "auth-header-map-secret",
									},
								},
								AuthDestHeader:       "Authorization",
								AuthDestHeaderPrefix: "Bearer ",
								TokenExpr:            "$.token",
								BodyData: v1alpha1.SecretKeyRef{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "auth-body-data-secret",
										},
										Key: "bodydata",
									},
								},
							},
						},
					},
				},
			},
			TLSConfig: &v1alpha1.TLSConfig{
				InsecureSkipVerify: false,
				CACerts: []corev1.SecretKeySelector{
					corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "tls-config-secret",
						},
						Key: "ca.crt",
					},
				},
				Certificates: []v1alpha1.Certificate{
					v1alpha1.Certificate{
						ClientCert: v1alpha1.SecretKeyRef{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "tls-config-secret",
								},
								Key: "tls.crt",
							},
						},
						ClientKey: v1alpha1.SecretKeyRef{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "tls-config-secret",
								},
								Key: "tls.key",
							},
						},
					},
				},
				CipherSuites: []string{"TLS_AES_128_GCM_SHA256",
					"TLS_AES_256_GCM_SHA384",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				MinVersion: "VersionTLS12",
			},
			ConfirmDelivery: ptr.Bool(false),
		},
	}
	Expect(k8sClient.Create(context.TODO(), dataReporterConfig)).Should(Succeed(), "create datareporterconfig")

	destHeaderMap := make(map[string]string)
	destHeaderMap["accept"] = "*/*"
	destHeaderSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dest-header-map-secret",
			Namespace: "default",
		},
		StringData: destHeaderMap,
	}
	err = k8sClient.Create(context.TODO(), &destHeaderSecret)
	Expect(err).ToNot(HaveOccurred())

	authHeaderMap := make(map[string]string)
	authHeaderMap["accept"] = "application/json"
	authHeaderMap["Content-Type"] = "application/json"
	authHeaderSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-header-map-secret",
			Namespace: "default",
		},
		StringData: authHeaderMap,
	}
	err = k8sClient.Create(context.TODO(), &authHeaderSecret)
	Expect(err).ToNot(HaveOccurred())

	authDataMap := make(map[string]string)
	authDataMap["bodydata"] = myApiKey
	authDataSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-body-data-secret",
			Namespace: "default",
		},
		StringData: authDataMap,
	}
	err = k8sClient.Create(context.TODO(), &authDataSecret)
	Expect(err).ToNot(HaveOccurred())

	kazaamMap := make(map[string]string)
	kazaamMap["kazaam.json"] = `[{"operation": "shift", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`
	kazaamConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kazaam-configmap",
			Namespace: "default",
		},
		Data: kazaamMap,
	}
	err = k8sClient.Create(context.TODO(), &kazaamConfigMap)
	Expect(err).ToNot(HaveOccurred())

	err = dataFilters.Build(dataReporterConfig)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	var err error

	httpTestServer.Close()
	grpcServer.Stop()
	eeCancel()

	os.RemoveAll(certDir)

	err = testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

//stop testEnv

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

func checkJSONBytesEqual(item1, item2 []byte) (bool, error) {
	var out1, out2 interface{}

	err := json.Unmarshal(item1, &out1)
	if err != nil {
		return false, nil
	}

	err = json.Unmarshal(item2, &out2)
	if err != nil {
		return false, nil
	}

	return reflect.DeepEqual(out1, out2), nil
}

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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

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
	eeCtx          context.Context
	eeCancel       context.CancelFunc
)

const (
	testData    = `{"event":"myevent"}`
	testDataBad = `{"event":"myevent"`
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

	httpTestServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// TODO: Server got transformed data
		fmt.Println("checking data")
		bodyBytes, err := io.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(Equal(testData))
		fmt.Println("good data")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, client")
	}))

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

	// DataFilters

	dataFilters = datafilter.NewDataFilters(ctrl.Log.WithName("datafilter"), k8sClient, httpTestClient)

	// k8s Configuration Objects

	dataReporterConfig := &v1alpha1.DataReporterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "datareporterconfig",
			Namespace: "default",
		},
		Spec: v1alpha1.DataReporterConfigSpec{},
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
	authDataMap["auth"] = `{"token": "eyJraWQiOiIx..."}`
	authDataSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-data-secrett",
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

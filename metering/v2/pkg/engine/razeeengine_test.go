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

package engine

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	//	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	testcase1 "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/test/razeeengine_testcase1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const clusterid = "MyClusterID1234567890"

var _ = Describe("RazeeEngineTest", func() {

	var (
		server *ghttp.Server
		//		body           []byte
		apipath string
		path    string
		//		err            error
		secret         *corev1.Secret
		clusterversion *openshiftconfigv1.ClusterVersion
		addr           string
		err            error
	)

	BeforeEach(func() {
		// start a test http server
		server = ghttp.NewTLSServer()
		server.SetAllowUnhandledRequests(true)

		apipath = "/api/collector/v2"

		// Expected full path of request
		path = apipath + "/clusters/" + clusterid + "/resources"

		// As per rhm-operator-secret RAZEE_DASH_URL
		// https://marketplace.redhat.com/api/collector/v2
		addr = "https://" + server.Addr() + apipath

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: "openshift-redhat-marketplace",
			},
			Data: map[string][]byte{
				utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
				utils.IBM_COS_URL_FIELD:        []byte("rhm-cos-url"),
				utils.BUCKET_NAME_FIELD:        []byte("bucket-name"),
				utils.RAZEE_DASH_ORG_KEY_FIELD: []byte("razee-dash-org-key"),
				utils.CHILD_RRS3_YAML_FIELD:    []byte("childRRS3-filename"),
				utils.RAZEE_DASH_URL_FIELD:     []byte(addr),
				utils.FILE_SOURCE_URL_FIELD:    []byte("file-source-url"),
			},
		}

		clusterversion = &openshiftconfigv1.ClusterVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: "version",
			},
			Spec: openshiftconfigv1.ClusterVersionSpec{
				ClusterID: clusterid,
			},
		}

		_, err = controllerutil.CreateOrUpdate(
			context.TODO(),
			k8sClient,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift-redhat-marketplace",
				}},
			func() error {
				return nil
			})
		Expect(err).Should(Succeed())

		/*
			Expect(k8sClient.Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift-redhat-marketplace",
				},
			})).Should(Succeed())
		*/
		Expect(k8sClient.Create(context.TODO(), secret)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), clusterversion)).Should(Succeed())

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", path),
				ghttp.RespondWith(http.StatusOK, ""),
				ghttp.VerifyHeader(http.Header{
					"razee-org-key": []string{"razee-dash-org-key"},
				}),
				ghttp.VerifyContentType("application/json"),
				//ghttp.VerifyBody(),
			),
		)

	})

	AfterEach(func() {
		server.Close()
		Expect(k8sClient.Delete(context.TODO(), secret)).Should(Succeed())
		Expect(k8sClient.Delete(context.TODO(), clusterversion)).Should(Succeed())
	})

	It("should process and send deployment create event", func() {
		Expect(k8sClient.Create(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
	})

	/*
		Expect(k8sClient.Create(context.TODO(), testcase1.ClusterServiceVersion)).Should(Succeed())


		Expect(k8sClient.Create(context.TODO(), testcase1.ClusterVersion)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Console.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Infrastructure.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.MarketplaceConfig.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Node.DeepCopy())).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), testcase1.Subscription.DeepCopy())).Should(Succeed())
	*/

})

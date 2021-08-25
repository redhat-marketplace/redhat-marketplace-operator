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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	//	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	testcase1 "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/test/razeeengine_testcase1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const clusterid = "MyClusterID1234567890"
const razeeOrgKey = "razee-dash-org-key"

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
		body           []byte
		bodychan       chan []byte
	)

	BeforeEach(func() {
		//Set Client to insecure
		os.Setenv("INSECURE_CLIENT", "true")

		// start a test http server
		server = ghttp.NewTLSServer()
		server.SetAllowUnhandledRequests(true)

		apipath = "/api/collector/v2"

		// Expected full path of request
		path = apipath + "/clusters/" + clusterid + "/resources"
		fmt.Println("--Path--")
		fmt.Println(path)

		// As per rhm-operator-secret RAZEE_DASH_URL
		// https://marketplace.redhat.com/api/collector/v2
		addr = "https://" + server.Addr() + apipath

		fmt.Println("--Addr--")
		fmt.Println(addr)

		//Send the body
		bodychan = make(chan []byte)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
				utils.IBM_COS_URL_FIELD:        []byte("rhm-cos-url"),
				utils.BUCKET_NAME_FIELD:        []byte("bucket-name"),
				utils.RAZEE_DASH_ORG_KEY_FIELD: []byte(razeeOrgKey),
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

		Expect(k8sClient.Create(context.TODO(), secret)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(), clusterversion)).Should(Succeed())

		server.RouteToHandler("POST", path, ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", path),
			ghttp.RespondWith(http.StatusOK, ""),
			ghttp.VerifyContentType("application/json"),

			ghttp.VerifyHeader(http.Header{
				"razee-org-key": []string{razeeOrgKey},
			}),

			func(w http.ResponseWriter, req *http.Request) {
				fmt.Println("---Got a Body---")
				body, err = ioutil.ReadAll(req.Body)
				req.Body.Close()
				bodychan <- body

				fmt.Println("---Header---")
				fmt.Print(req.Header)
			},
		))
	})

	AfterEach(func() {
		close(bodychan)
		server.Close()
		Expect(k8sClient.Delete(context.TODO(), secret)).Should(Succeed())
		Expect(k8sClient.Delete(context.TODO(), clusterversion)).Should(Succeed())
		fmt.Println("---Body---")
		fmt.Print(string(body))
	})

	It("should process and send deployment CRUD events", func() {
		Expect(k8sClient.Create(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
		Eventually(deploymentCheck(bodychan, watch.Added), "30s").Should(Equal(true))

		Expect(k8sClient.Delete(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
		Eventually(deploymentCheck(bodychan, watch.Deleted), "30s").Should(Equal(true))

	})

})

func deploymentCheck(bodychan chan []byte, expectedEventType watch.EventType) bool {
	for body := range bodychan {
		fmt.Println("---DC Body---")
		fmt.Print(string(body))

		var eventObjects []map[string]interface{}
		if err := json.Unmarshal(body, &eventObjects); err != nil {
			fmt.Println("---Error Unmarshal eventObjects---")
			fmt.Println(err)
			return false
		}

		for _, eventObject := range eventObjects {
			// Event Type matches expected
			fmt.Println("---Event Type---")
			fmt.Println(eventObject["type"])
			eventType, ok := eventObject["type"].(string)
			if !ok {
				fmt.Println("---Could not assert EventType---")
				return false
			}
			if eventType != string(expectedEventType) {
				fmt.Println("---Event Type did not match expected---")
				continue
			}

			// Object content
			object, err := json.Marshal(eventObject["object"])
			if err != nil {
				fmt.Println("---Error Marshall object---")
				fmt.Println(err)
				return false
			}

			// GVK is Deployment
			var gvkobject map[string]interface{}
			if err := json.Unmarshal(object, &gvkobject); err != nil {
				fmt.Println("---Error Unmarshal gvkobject---")
				fmt.Println(err)
				return false
			}

			if (gvkobject["kind"] != "Deployment") || (gvkobject["apiVersion"] != "apps/v1") {
				fmt.Println("---Not an apps/v1 Deployment---")
				continue
			}

			// Object is a Deployment
			var deployment appsv1.Deployment
			err = json.Unmarshal(object, &deployment)
			if err != nil {
				fmt.Println("---Error Unmarshal deployment---")
				fmt.Println(err)
				continue
			}

			// Deployment is the test Deployment, and has been sanitized
			if deployment.Name == "rhm-remoteresources3-controller" && deployment.Namespace == "openshift-redhat-marketplace" {
				// Annotations are sanitized
				annotations := deployment.GetAnnotations()
				if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
					fmt.Println("--Has Annotation")
					return false
				}
				if _, ok := annotations["kapitan.razee.io/last-applied-configuration"]; ok {
					fmt.Println("--Has Annotation")
					return false
				}
				if _, ok := annotations["deploy.razee.io/last-applied-configuration"]; ok {
					fmt.Println("--Has Annotation")
					return false
				}

				// Env is sanitized
				//emptyVar := []corev1.EnvVar{}
				for i, _ := range deployment.Spec.Template.Spec.Containers {
					if len(deployment.Spec.Template.Spec.Containers[i].Env) != 0 {
						fmt.Println("--Env not empty")
						return false
					}
				}
				fmt.Println("---Found Matching Deployment---")
				return true
			}
		}
	}
	return false
}

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

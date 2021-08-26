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
	"time"

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
		ctx            context.Context
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

		// As per rhm-operator-secret RAZEE_DASH_URL
		// https://marketplace.redhat.com/api/collector/v2
		addr = "https://" + server.Addr() + apipath

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
				body, err = ioutil.ReadAll(req.Body)
				req.Body.Close()
				bodychan <- body
			},
		))
		ctx = context.Background()
		ctx, _ = context.WithTimeout(ctx, 30*time.Second)
	})

	AfterEach(func() {
		close(bodychan)
		server.Close()
		Expect(k8sClient.Delete(context.TODO(), secret)).Should(Succeed())
		Expect(k8sClient.Delete(context.TODO(), clusterversion)).Should(Succeed())
	})

	It("should process and send deployment CRUD events", func() {
		deploymentResult := map[string]bool{
			"isEventObjects":        true,
			"isEventType":           true,
			"isExpectedEventType":   true,
			"containsObject":        true,
			"isGVKObject":           true,
			"gvkIsDeployment":       true,
			"isDeployment":          true,
			"isNameNamespace":       true,
			"isAnnotationSanitized": true,
			"isEnvSanitized":        true,
		}

		Expect(k8sClient.Create(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
		Eventually(deploymentCheck(ctx, bodychan, watch.Added), "30s").Should(Equal(deploymentResult))

		Expect(k8sClient.Update(context.TODO(), testcase1.DeploymentUpdated.DeepCopy())).Should(Succeed())
		Eventually(deploymentCheck(ctx, bodychan, watch.Modified), "30s").Should(Equal(deploymentResult))

		Expect(k8sClient.Delete(context.TODO(), testcase1.Deployment.DeepCopy())).Should(Succeed())
		Eventually(deploymentCheck(ctx, bodychan, watch.Deleted), "30s").Should(Equal(deploymentResult))

	})

	It("should process and send node event", func() {
		nodeResult := map[string]bool{
			"isEventObjects":        true,
			"isEventType":           true,
			"isExpectedEventType":   true,
			"containsObject":        true,
			"isGVKObject":           true,
			"gvkIsNode":             true,
			"isNode":                true,
			"isAnnotationSanitized": true,
		}

		Expect(k8sClient.Create(context.TODO(), testcase1.Node.DeepCopy())).Should(Succeed())
		Eventually(nodeCheck(ctx, bodychan, watch.Added), "30s").Should(Equal(nodeResult))

	})

})

func nodeCheck(ctx context.Context, bodychan chan []byte, expectedEventType watch.EventType) map[string]bool {

	result := make(map[string]bool)

	// Return the result on a catastrophic failure such as Marshal failure
	// Continue to next object if object does not match expected Deployment

	select {
	case <-ctx.Done():
		close(bodychan)
		return result
	case body := <-bodychan:

		fmt.Printf("body: %+v", string(body))

		// isEventObjects
		var eventObjects []map[string]interface{}
		if err := json.Unmarshal(body, &eventObjects); err != nil {
			result["isEventObjects"] = false
			return result
		}
		result["isEventObjects"] = true

		for _, eventObject := range eventObjects {

			// isEventType
			eventType, ok := eventObject["type"].(string)
			if !ok {
				result["isEventType"] = false
				return result
			}
			result["isEventType"] = true

			// isExpectedEventType
			if eventType != string(expectedEventType) {
				result["isExpectedEventType"] = false
				continue
			}
			result["isExpectedEventType"] = true

			// containsObject
			object, err := json.Marshal(eventObject["object"])
			if err != nil {
				result["containsObject"] = false
				return result
			}
			result["containsObject"] = true

			// isGVKObject
			var gvkobject map[string]interface{}
			if err := json.Unmarshal(object, &gvkobject); err != nil {
				result["isGVKObject"] = false
				return result
			}
			result["isGVKObject"] = true

			// gvkIsNode
			if (gvkobject["kind"] != "Node") || (gvkobject["apiVersion"] != "v1") {
				result["gvkIsNode"] = false
				continue
			}
			result["gvkIsNode"] = true

			// isNode
			var node corev1.Node
			err = json.Unmarshal(object, &node)
			if err != nil {
				result["isNode"] = false
				continue
			}
			result["isNode"] = true

			// isAnnotationSanitized
			annotations := node.GetAnnotations()
			if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			if _, ok := annotations["kapitan.razee.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			if _, ok := annotations["deploy.razee.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			result["isAnnotationSanitized"] = true

		}
	}
	return result
}

func deploymentCheck(ctx context.Context, bodychan chan []byte, expectedEventType watch.EventType) map[string]bool {

	result := make(map[string]bool)

	// Return the result on a catastrophic failure such as Marshal failure
	// Continue to next object if object does not match expected Deployment

	select {
	case <-ctx.Done():
		close(bodychan)
		return result
	case body := <-bodychan:
		//	for body := range bodychan {

		// isEventObjects
		var eventObjects []map[string]interface{}
		if err := json.Unmarshal(body, &eventObjects); err != nil {
			result["isEventObjects"] = false
			return result
		}
		result["isEventObjects"] = true

		for _, eventObject := range eventObjects {

			// isEventType
			eventType, ok := eventObject["type"].(string)
			if !ok {
				result["isEventType"] = false
				return result
			}
			result["isEventType"] = true

			// isExpectedEventType
			if eventType != string(expectedEventType) {
				result["isExpectedEventType"] = false
				continue
			}
			result["isExpectedEventType"] = true

			// containsObject
			object, err := json.Marshal(eventObject["object"])
			if err != nil {
				result["containsObject"] = false
				return result
			}
			result["containsObject"] = true

			// isGVKObject
			var gvkobject map[string]interface{}
			if err := json.Unmarshal(object, &gvkobject); err != nil {
				result["isGVKObject"] = false
				return result
			}
			result["isGVKObject"] = true

			// gvkIsDeployment
			if (gvkobject["kind"] != "Deployment") || (gvkobject["apiVersion"] != "apps/v1") {
				result["gvkIsDeployment"] = false
				continue
			}
			result["gvkIsDeployment"] = true

			// isDeployment
			var deployment appsv1.Deployment
			err = json.Unmarshal(object, &deployment)
			if err != nil {
				result["isDeployment"] = false
				continue
			}
			result["isDeployment"] = true

			// isNameNamespace
			if (deployment.Name != "rhm-remoteresources3-controller") || (deployment.Namespace != "openshift-redhat-marketplace") {
				result["isNameNamespace"] = false
				continue
			}
			result["isNameNamespace"] = true

			// isAnnotationSanitized
			annotations := deployment.GetAnnotations()
			if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			if _, ok := annotations["kapitan.razee.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			if _, ok := annotations["deploy.razee.io/last-applied-configuration"]; ok {
				result["isAnnotationSanitized"] = false
				continue
			}
			result["isAnnotationSanitized"] = true

			// isEnvSanitized
			isEnvSanitized := true
			for i, _ := range deployment.Spec.Template.Spec.Containers {
				if len(deployment.Spec.Template.Spec.Containers[i].Env) != 0 {
					isEnvSanitized = false
				}
			}
			result["isEnvSanitized"] = isEnvSanitized
		}
	}
	return result
}

/*  Other types to potentially test for
Expect(k8sClient.Create(context.TODO(), testcase1.ClusterServiceVersion)).Should(Succeed())
Expect(k8sClient.Create(context.TODO(), testcase1.ClusterVersion)).Should(Succeed())
Expect(k8sClient.Create(context.TODO(), testcase1.Console.DeepCopy())).Should(Succeed())
Expect(k8sClient.Create(context.TODO(), testcase1.Infrastructure.DeepCopy())).Should(Succeed())
Expect(k8sClient.Create(context.TODO(), testcase1.MarketplaceConfig.DeepCopy())).Should(Succeed())
Expect(k8sClient.Create(context.TODO(), testcase1.Subscription.DeepCopy())).Should(Succeed())
*/

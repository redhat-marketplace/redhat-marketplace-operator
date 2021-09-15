// Copyright 2020 IBM Corp.
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

package utils

import (
	"encoding/json"
	"reflect"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("add secret fields to struct", func() {

		instance := marketplacev1alpha1.RazeeDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: "redhat-marketplace-operator",
			},
			Spec: marketplacev1alpha1.RazeeDeploymentSpec{
				Enabled:     true,
				ClusterUUID: "test-uuid",
				DeployConfig: &marketplacev1alpha1.RazeeConfigurationValues{
					IbmCosReaderKey: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: RHM_OPERATOR_SECRET_NAME,
						},
						Key: IBM_COS_READER_KEY_FIELD,
					},
					BucketName: TEST_BUCKET_NAME_FIELD,
					IbmCosURL:  TEST_IBM_COS_URL_FIELD,
					RazeeDashOrgKey: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: RHM_OPERATOR_SECRET_NAME,
						},
						Key: RAZEE_DASH_ORG_KEY_FIELD,
					},
					ChildRSS3FIleName: TEST_CHILD_RRS3_YAML_FIELD,
					RazeeDashUrl:      TEST_RAZEE_DASH_URL_FIELD,
					FileSourceURL:     ptr.String(TEST_FILE_SOURCE_URL_FIELD),
				},
			},
		}

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: "redhat-marketplace-operator",
			},
			Data: map[string][]byte{
				IBM_COS_READER_KEY_FIELD: []byte(TEST_IBM_COS_READER_KEY_FIELD),
				IBM_COS_URL_FIELD:        []byte(TEST_IBM_COS_URL_FIELD),
				BUCKET_NAME_FIELD:        []byte(TEST_BUCKET_NAME_FIELD),
				RAZEE_DASH_ORG_KEY_FIELD: []byte(TEST_RAZEE_DASH_ORG_KEY_FIELD),
				CHILD_RRS3_YAML_FIELD:    []byte(TEST_CHILD_RRS3_YAML_FIELD),
				RAZEE_DASH_URL_FIELD:     []byte(TEST_RAZEE_DASH_URL_FIELD),
			},
		}

		secretWithMissingValue := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: "redhat-marketplace-operator",
			},

			Data: map[string][]byte{
				IBM_COS_READER_KEY_FIELD: []byte(TEST_IBM_COS_READER_KEY_FIELD),
				IBM_COS_URL_FIELD:        []byte(TEST_IBM_COS_URL_FIELD),
				BUCKET_NAME_FIELD:        []byte(TEST_BUCKET_NAME_FIELD),
				RAZEE_DASH_ORG_KEY_FIELD: []byte(TEST_RAZEE_DASH_ORG_KEY_FIELD),
				CHILD_RRS3_YAML_FIELD:    []byte(TEST_CHILD_RRS3_YAML_FIELD),
			},
		}

		expectedDeployConfigValues := marketplacev1alpha1.RazeeConfigurationValues{
			IbmCosReaderKey: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RHM_OPERATOR_SECRET_NAME,
				},
				Key: IBM_COS_READER_KEY_FIELD,
			},
			BucketName: "bucket-name",
			IbmCosURL:  "ibm-cos-url",
			RazeeDashOrgKey: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RHM_OPERATOR_SECRET_NAME,
				},
				Key: RAZEE_DASH_ORG_KEY_FIELD,
			},
			ChildRSS3FIleName: "childRRS3-filename",
			RazeeDashUrl:      "razee-dash-url",
			FileSourceURL:     ptr.String("file-source-url"),
		}

		// test that it returns the correct format if all keys are present
		returnedRazeeConfigValues, missingItems, err := AddSecretFieldsToStruct(secret.Data, instance)
		if !reflect.DeepEqual(returnedRazeeConfigValues, expectedDeployConfigValues) {
			returnedRazeeConfigValuesJson, _ := json.MarshalIndent(returnedRazeeConfigValues, "", "    ")
			expectedDeployConfigValuesJson, _ := json.MarshalIndent(expectedDeployConfigValues, "", "    ")
			GinkgoT().Errorf("AddSecretFieldsToStruct returned\n %v\n should have returned\n %v", string(returnedRazeeConfigValuesJson), string(expectedDeployConfigValuesJson))
		}

		if len(missingItems) != 0 {
			GinkgoT().Errorf("missingItems should be empty. Returned: %v", missingItems)
		}

		if err != nil {
			GinkgoT().Errorf("failed with error %v", err)
		}

		// test that AddSecretFieldsToStruct appends the correct missing value if a secret is missing a field
		_, missingItems, err = AddSecretFieldsToStruct(secretWithMissingValue.Data, instance)

		if !Contains(missingItems, RAZEE_DASH_URL_FIELD) {
			GinkgoT().Errorf("missingItems should contain missing field %v", RAZEE_DASH_URL_FIELD)
		}

		if err != nil {
			GinkgoT().Errorf("failed with error %v", err)
		}

		// test that if a field is missing from rhm-operator-secret that the struct value on Spec.DeployConfig doesn't get set to nil/omitted
		returnedRazeeConfigValues, missingItems, err = AddSecretFieldsToStruct(secretWithMissingValue.Data, instance)
		PrettyPrint(returnedRazeeConfigValues)
		if returnedRazeeConfigValues.RazeeDashUrl != TEST_RAZEE_DASH_URL_FIELD {
			GinkgoT().Errorf("RazeeConfigurationValues.RazeeDashUrl overwritten")
		}

		if err != nil {
			GinkgoT().Errorf("failed with error %v", err)
		}
	})
	It("apply annotation", func() {

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "myns",
			},
		}

		ApplyAnnotation(cm)
		assert.Contains(GinkgoT(), cm.ObjectMeta.Annotations, RhmAnnotationKey, "Annotations does not contain key")
	})

	It("should chunk by", func() {
		Expect(ChunkBy([]interface{}{}, 2)).To(HaveLen(0))
		Expect(ChunkBy([]interface{}{"a", "b"}, 2)).To(HaveLen(1))
		Expect(ChunkBy([]interface{}{"a", "b", nil, nil}, 2)).To(HaveLen(2))
		chunks := ChunkBy([]interface{}{"a", "b", "c", "d", "e", "f"}, 2)
		Expect(chunks).To(HaveLen(3))
		Expect(chunks[0]).To(Equal([]interface{}{"a", "b"}))
		Expect(chunks[1]).To(Equal([]interface{}{"c", "d"}))
		Expect(chunks[2]).To(Equal([]interface{}{"e", "f"}))
		Expect(func() {
			ChunkBy([]interface{}{"a", "b", nil}, 2)
		}).To(PanicWith("items length is not chunkable by the size"))
	})

	It("Should return a list of meterdefs to be deleted", func(){
		testMeterdef1 := marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "meterdef-1",
				Namespace: "namespace",
			},
		}

		testMeterdef2 := marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "meterdef-2",
				Namespace: "namespace",
			},
		}

		meterdefsOnCluster := []marketplacev1beta1.MeterDefinition{testMeterdef1,testMeterdef2}
		latestFromCatalog := []marketplacev1beta1.MeterDefinition{testMeterdef1}
		diff := FindMeterdefSliceDiff(meterdefsOnCluster,latestFromCatalog)
		Expect(len(diff)).To(Equal(1))
		Expect(diff[0].Name).To(Equal("meterdef-2"))

		latestFromCatalog = append(latestFromCatalog, testMeterdef2)
		diff = FindMeterdefSliceDiff(meterdefsOnCluster,latestFromCatalog)
		Expect(len(diff)).To(Equal(0))
	})
})

const (
	TEST_IBM_COS_READER_KEY_FIELD = "ibm-cos-reader-key"
	TEST_IBM_COS_URL_FIELD        = "ibm-cos-url"
	TEST_BUCKET_NAME_FIELD        = "bucket-name"
	TEST_RAZEE_DASH_ORG_KEY_FIELD = "razee-dash-org-key"
	TEST_CHILD_RRS3_YAML_FIELD    = "childRRS3-filename"
	TEST_RAZEE_DASH_URL_FIELD     = "razee-dash-url"
	TEST_FILE_SOURCE_URL_FIELD    = "file-source-url"
)

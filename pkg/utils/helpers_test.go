package utils

import (
	"encoding/json"
	"reflect"
	"testing"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TEST_IBM_COS_READER_KEY_FIELD = "ibm-cos-reader-key"
	TEST_IBM_COS_URL_FIELD        = "ibm-cos-url"
	TEST_BUCKET_NAME_FIELD        = "bucket-name"
	TEST_RAZEE_DASH_ORG_KEY_FIELD = "razee-dash-org-key"
	TEST_CHILD_RRS3_YAML_FIELD    = "childRRS3-filename"
	TEST_RAZEE_DASH_URL_FIELD     = "razee-dash-url"
	TEST_FILE_SOURCE_URL_FIELD    = "file-source-url"
)

func TestAddSecretFieldsToStruct(t *testing.T) {

	instance := marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-operator-secret",
			Namespace: "redhat-marketplace-operator",
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled: true,
			ClusterUUID: "test-uuid",
			DeployConfig: &marketplacev1alpha1.RazeeConfigurationValues{
				IbmCosReaderKey: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: RHM_OPERATOR_SECRET_NAME,
					},
					Key: IBM_COS_READER_KEY_FIELD,
				},
				BucketName: TEST_BUCKET_NAME_FIELD,
				IbmCosURL: TEST_IBM_COS_URL_FIELD,
				RazeeDashOrgKey: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: RHM_OPERATOR_SECRET_NAME,
					},
					Key: RAZEE_DASH_ORG_KEY_FIELD,
				},
				ChildRSS3FIleName: TEST_CHILD_RRS3_YAML_FIELD,
				RazeeDashUrl: TEST_RAZEE_DASH_URL_FIELD,
				FileSourceURL: TEST_FILE_SOURCE_URL_FIELD,
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
			FILE_SOURCE_URL_FIELD:    []byte(TEST_FILE_SOURCE_URL_FIELD),
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
			RAZEE_DASH_URL_FIELD:     []byte(TEST_RAZEE_DASH_URL_FIELD),
			// FILE_SOURCE_URL_FIELD:    []byte("file-source-url"),
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
		RazeeDashUrl:     "razee-dash-url",
		FileSourceURL:    "file-source-url",
	}

	// test that it returns the correct format if all keys are present
	returnedRazeeConfigValues, missingItems, err := AddSecretFieldsToStruct(secret.Data, instance)
	if !reflect.DeepEqual(returnedRazeeConfigValues, expectedDeployConfigValues) {
		returnedRazeeConfigValuesJson, _ := json.MarshalIndent(returnedRazeeConfigValues, "", "    ")
		expectedDeployConfigValuesJson, _ := json.MarshalIndent(expectedDeployConfigValues, "", "    ")
		t.Errorf("AddSecretFieldsToStruct returned\n %v\n should have returned\n %v", string(returnedRazeeConfigValuesJson), string(expectedDeployConfigValuesJson))
	}

	if len(missingItems) != 0 {
		t.Errorf("missingItems should be empty. Returned: %v", missingItems)
	}

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	// test that it appends the correct missing value if the secret is missing a field
	_, missingItems, err = AddSecretFieldsToStruct(secretWithMissingValue.Data,instance)

	if !Contains(missingItems, FILE_SOURCE_URL_FIELD) {
		t.Errorf("missingItems should be contain missing field %v", FILE_SOURCE_URL_FIELD)
	}

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	// test that if a field is missing from the secret that struct value on Spec.DeployConfig doesn't get set to nil/omitted
	returnedRazeeConfigValues, missingItems, err = AddSecretFieldsToStruct(secretWithMissingValue.Data,instance)

	if returnedRazeeConfigValues.FileSourceURL != TEST_FILE_SOURCE_URL_FIELD {
		t.Errorf("RazeeConfigurationValues.FileSourceURL overwritten")
	}

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

}
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
	"context"
	b64 "encoding/base64"
	"fmt"
	"strings"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const RhmAnnotationKey = "marketplace.redhat.com/last-applied"

var RhmAnnotator = patch.NewAnnotator(RhmAnnotationKey)
var RhmPatchMaker = patch.NewPatchMaker(RhmAnnotator)

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}

func ChunkBy(items []interface{}, chunkSize int) (chunks [][]interface{}) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}

	return append(chunks, items)
}

func ContainsMultiple(inArray []string, referenceArray []string) []string {
	var temp []string
	for _, searchItem := range referenceArray {
		if !Contains(inArray, searchItem) {
			temp = append(temp, searchItem)
		}

	}
	return temp
}

// Remove() will remove the key from the list
func RemoveKey(list []string, key string) []string {
	newList := []string{}
	for _, s := range list {
		if s != key {
			newList = append(newList, s)
		}
	}
	return newList
}

func RetrieveSecretField(in []byte) (string, error) {
	decodedString := b64.StdEncoding.EncodeToString(in)
	decoded, err := b64.StdEncoding.DecodeString(decodedString)

	return strings.Trim(string(decoded), " \r\n"), err
}

func ExtractCredKey(secret *corev1.Secret, sel corev1.SecretKeySelector) ([]byte, error) {
	var value []byte
	var error error
	if value, ok := secret.Data[sel.Key]; ok {
		return value, nil
	} else if !ok {
		error = fmt.Errorf("secret %s key %q not in secret", sel.Key, sel.Name)
	}

	return value, error
}

func GetDataFromRhmSecret(request reconcile.Request, sel corev1.SecretKeySelector, client client.Client) (error, []byte) {
	// get the operator secret
	rhmOperatorSecret := corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      RHM_OPERATOR_SECRET_NAME,
		Namespace: request.Namespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return err, nil
		}
		return err, nil
	}
	key, err := ExtractCredKey(&rhmOperatorSecret, sel)
	return err, key
}

func AddSecretFieldsToStruct(razeeData map[string][]byte, instance marketplacev1alpha1.RazeeDeployment) (marketplacev1alpha1.RazeeConfigurationValues, []string, error) {
	// var razeeStruct *marketplacev1alpha1.RazeeConfigurationValues = &marketplacev1alpha1.RazeeConfigurationValues{}
	if instance.Spec.DeployConfig == nil {
		instance.Spec.DeployConfig = &marketplacev1alpha1.RazeeConfigurationValues{}
	}

	razeeStruct := instance.Spec.DeployConfig
	keys := []string{}
	expectedKeys := []string{
		IBM_COS_URL_FIELD,
		BUCKET_NAME_FIELD,
		IBM_COS_URL_FIELD,
		RAZEE_DASH_ORG_KEY_FIELD,
		CHILD_RRS3_YAML_FIELD,
		RAZEE_DASH_URL_FIELD,
		FILE_SOURCE_URL_FIELD,
	}

	for key, element := range razeeData {
		keys = append(keys, key)
		value, err := RetrieveSecretField(element)
		if err != nil {
			//TODO: better way to handle this here?
			razeeStruct = nil
			return marketplacev1alpha1.RazeeConfigurationValues{}, nil, err
		}

		switch key {
		case IBM_COS_READER_KEY_FIELD:
			razeeStruct.IbmCosReaderKey = &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RHM_OPERATOR_SECRET_NAME,
				},
				Key: key,
			}

		case BUCKET_NAME_FIELD:
			razeeStruct.BucketName = value

		case IBM_COS_URL_FIELD:
			razeeStruct.IbmCosURL = value

		case RAZEE_DASH_ORG_KEY_FIELD:
			razeeStruct.RazeeDashOrgKey = &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RHM_OPERATOR_SECRET_NAME,
				},
				Key: key,
			}

		case CHILD_RRS3_YAML_FIELD:
			razeeStruct.ChildRSS3FIleName = value

		case RAZEE_DASH_URL_FIELD:
			razeeStruct.RazeeDashUrl = value

		case FILE_SOURCE_URL_FIELD:
			razeeStruct.FileSourceURL = value

		}
	}

	missingItems := ContainsMultiple(keys, expectedKeys)
	return *razeeStruct, missingItems, nil
}

func ApplyAnnotation(resource runtime.Object) error {
	return RhmAnnotator.SetLastAppliedAnnotation(resource)
}

func Equal(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func ConditionsEqual(a status.Conditions, b status.Conditions) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

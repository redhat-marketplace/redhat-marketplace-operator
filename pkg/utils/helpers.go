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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func GetNamespaceNames(ns []corev1.Namespace) []string {
	var namespaceNames []string
	for _, namespace := range ns {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	return namespaceNames
}

func GetSecretNames(secretList []corev1.Secret) []string {
	var secretNames []string
	for _, secret := range secretList {
		secretNames = append(secretNames, secret.Name)
	}

	return secretNames
}

func GetConfigMapNames(configMapList []corev1.ConfigMap) []string {
	var configMapNames []string
	for _, configMap := range configMapList {
		configMapNames = append(configMapNames, configMap.Name)
	}

	return configMapNames
}

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

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
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

func CheckMapKeys(razeeConfigValues map[string]string, referenceList []string) []string {
	missingItems := []string{}
	for _, referenceItem := range referenceList {
		if _, exists := razeeConfigValues[referenceItem]; !exists {
			missingItems = append(missingItems, referenceItem)
		}
	}
	return missingItems
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

func AddSecretFieldsToObj(razeeData map[string][]byte) (map[string]string, error) {
	razeeDataObj := make(map[string]string)
	var error error
	for key, element := range razeeData {
		value, err := RetrieveSecretField(element)
		razeeDataObj[key] = value
		if err != nil {
			error = err
		}
	}

	return razeeDataObj, error
}

//TODO: not being used
func AddSecretFieldsToStruct(razeeData map[string][]byte) (*marketplacev1alpha1.RazeeConfigurationValues, []string, error) {

	newField := []byte{}
	var razeeStruct *marketplacev1alpha1.RazeeConfigurationValues = &marketplacev1alpha1.RazeeConfigurationValues{}
	var error error
	keys := []string{}
	for key, element := range razeeData {
		keys = append(keys, key)
		value, err := RetrieveSecretField(element)
		if err != nil {
			error = err
		}

		// fullfill the SecretKeySelector struct
		if key == "IBM_COS_READER_KEY" {
			newField = []byte(fmt.Sprintf(
				`{"%v": {"name": "rhm-operator-secret","key": "%v"}}`, key, key))
		} else if key == "RAZEE_DASH_ORG_KEY" {
			newField = []byte(fmt.Sprintf(
				`{"%v": {"name": "rhm-operator-secret","key": "%v"}}`, key, key))
		} else {
			newField = []byte(fmt.Sprintf(`{"%v": "%v"}`, key, value))
		}

		// add to the struct
		err = json.Unmarshal(newField, &razeeStruct)
		if err != nil {
			error = err
		}
		if err != nil {
			error = err
		}
	}

	missingItems := ContainsMultiple(keys, GetStructFieldsOnBase(marketplacev1alpha1.RazeeConfigurationValues{}))
	return razeeStruct, missingItems, error
}

// returns the fields on the type/base struct so we don't to maintain a list of all the fields
func GetStructFieldsOnBase(razeeStruct marketplacev1alpha1.RazeeConfigurationValues) []string {
	aa := reflect.Indirect(reflect.ValueOf(razeeStruct))
	fieldSlice := []string{}
	for i := 0; i < aa.NumField(); i++ {
		field := aa.Type().Field(i)
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
				fieldName := jsonTag[:commaIdx]
				fieldSlice = append(fieldSlice, string(fieldName))
			}
		}
	}
	return fieldSlice
}

func ConvertSecretToStruct(razeeData map[string][]byte) (marketplacev1alpha1.RazeeConfigurationValues, []string, error) {

	newField := []byte{}
	var razeeStruct *marketplacev1alpha1.RazeeConfigurationValues = &marketplacev1alpha1.RazeeConfigurationValues{}
	var error error
	keys := []string{}
	for key, element := range razeeData {
		keys = append(keys, key)
		value, err := RetrieveSecretField(element)
		if err != nil {
			error = err
		}

		// fullfill the SecretKeySelector struct
		if key == "IBM_COS_READER_KEY" {
			newField = []byte(fmt.Sprintf(
				`{"%v": {"name": "rhm-operator-secret","key": "%v"}}`, key, key))
		} else if key == "RAZEE_DASH_ORG_KEY" {
			newField = []byte(fmt.Sprintf(
				`{"%v": {"name": "rhm-operator-secret","key": "%v"}}`, key, key))
		} else {
			newField = []byte(fmt.Sprintf(`{"%v": "%v"}`, key, value))
		}

		// add to the struct
		err = json.Unmarshal(newField, &razeeStruct)
		if err != nil {
			fmt.Println("AddSecretFieldsToStruct", err)
			error = err
		}
		if err != nil {
			error = err
		}
	}

	missingItems := ContainsMultiple(keys, GetStructFieldsOnBase(marketplacev1alpha1.RazeeConfigurationValues{}))
	return *razeeStruct, missingItems, error
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

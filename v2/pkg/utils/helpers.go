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
	json "encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/blang/semver/v4"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const RhmAnnotationKey = "marketplace.redhat.com/last-applied"

var RhmAnnotator = patch.NewAnnotator(RhmAnnotationKey)

var ParsedVersion460, _ = semver.Make("4.6.0")
var ParsedVersion480, _ = semver.Make("4.8.0")

func IsNil(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
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
	if len(items) == 0 {
		return
	}

	if len(items)%chunkSize != 0 {
		panic("items length is not chunkable by the size")
	}

	chunk := make([]interface{}, chunkSize, chunkSize)

	for i := 0; i < len(items); i = i + chunkSize {
		lowBound := i
		upperBound := lowBound + chunkSize

		if upperBound == len(items) {
			chunk = items[lowBound:]
		} else {
			chunk = items[lowBound:upperBound]
		}

		chunks = append(chunks, chunk)
	}

	return
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

func FindDiff(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
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

func ApplyAnnotation(resource client.Object) error {
	return RhmAnnotator.SetLastAppliedAnnotation(resource)
}

func StringSliceEqual(a []string, b []string) bool {
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

// AppendResourceList() returns the the combined ResourceList
func AppendResourceList(list1 corev1.ResourceList, list2 corev1.ResourceList) corev1.ResourceList {
	result := corev1.ResourceList{}
	for k, v := range list1 {
		if _, exists := list2[k]; !exists {
			list2[k] = v
		}
	}
	return result
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

func FindMeterdefSliceDiff(catalogMdefsOnCluster []marketplacev1beta1.MeterDefinition, latestMeterdefsFromCatalog []marketplacev1beta1.MeterDefinition) (deleteList []marketplacev1beta1.MeterDefinition) {
	for _, installedMeterdef := range catalogMdefsOnCluster {
		found := false
		for _, meterdefFromCatalog := range latestMeterdefsFromCatalog {

			if installedMeterdef.Name == meterdefFromCatalog.Name {
				found = true
				break
			}
		}

		if !found {
			deleteList = append(deleteList, installedMeterdef)
		}
	}

	return deleteList
}

func PrettyPrint(in interface{}) {
	out, _ := json.MarshalIndent(in, "", "    ")
	println(string(out))
}

func PrettyPrintWithLog(in interface{}, message string) {
	indented, _ := json.MarshalIndent(in, "", "    ")

	if message != "" {
		out := fmt.Sprintf("%s\n%s", message, indented)
		fmt.Println(string(out))
	} else {
		fmt.Println(string(indented))
	}
}

func TruncateTime(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

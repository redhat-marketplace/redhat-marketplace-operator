package utils

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

func AddSecretFieldsToStruct(razeeData map[string][]byte) (*marketplacev1alpha1.RazeeDeployConfig,[]string,error) {
	var updatedRazeeValues *marketplacev1alpha1.RazeeDeployConfig = &marketplacev1alpha1.RazeeDeployConfig{}
	missingItems := []string{}
	var error error

	for key, element := range razeeData {
		value, err := RetrieveSecretField(element)
		if err != nil {
			error = err
		}
		//TODO: need to think about this
		if value == "" {
			missingItems = append(missingItems, key)
		}

		newField := []byte(fmt.Sprintf(`{"%v": "%v"}`,key,value))
		err = json.Unmarshal(newField, &updatedRazeeValues)
		if err != nil {
			fmt.Println(err)
			error = err
		}
		fmt.Println(updatedRazeeValues)
		if err != nil {
			error = err
		}
	}

	return updatedRazeeValues,missingItems, error
}

func ConvertSecretToStruct(razeeData map[string][]byte)(marketplacev1alpha1.RazeeDeployConfig,[]string,error){
	input,err := AddSecretFieldsToObj(razeeData)

	fmt.Printf("input %#v\n", input)
	var md mapstructure.Metadata
	var result marketplacev1alpha1.RazeeDeployConfig
	config := &mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   &result,
		TagName: "json",
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		panic(err)
	}
	if err := decoder.Decode(input); err != nil {
		panic(err)
	}
	fmt.Printf("Unused keys: %#v\n", md.Unused)

	fmt.Printf("result %#v\n", result)

	return result, md.Unused,err
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

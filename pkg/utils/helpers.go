package utils

import (
	b64 "encoding/base64"
	"strings"

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

// Contains() checks if the lsit contains the key, if so - return it
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
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

func GetMapKeys(razeeConfigValues map[string]string)([]string){
	keys := make([]string, 0, len(razeeConfigValues))
    for value, _ := range razeeConfigValues {
        keys = append(keys, value)
	}
	return keys
}

func RetrieveSecretField(in []byte) (string, error) {
	decodedString := b64.StdEncoding.EncodeToString(in)
	decoded, err := b64.StdEncoding.DecodeString(decodedString)

	return strings.Trim(string(decoded), " \r\n"), err
}

func AddSecretFieldsToObj(razeeData map[string][]byte) (map[string]string, error) {
	// keys := []string{"IBM_COS_READER_KEY","BUCKET_NAME", "IBM_COS_URL","RAZEEDASH_ORG_KEY"}
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
// func GetFieldNames(razeeConfigValuesStruct *marketplacev1alpha1.RazeeConfigValues)([]string){
// 	keySlice := []string{}

//     e := reflect.ValueOf(razeeConfigValuesStruct).Elem()
//     indirected := reflect.Indirect(e)
//     for i := 0; i < indirected.NumField(); i++ {
//         varName := indirected.Type().Field(i).Name
//         keySlice = append(keySlice,varName)
// 	}
// 	return keySlice
// }

// func AddSecretFieldsToStruct(razeeData map[string][]byte) (*marketplacev1alpha1.RazeeConfigValues,[]string,error) {
// 	var updatedRazeeValues *marketplacev1alpha1.RazeeConfigValues = &marketplacev1alpha1.RazeeConfigValues{}
// 	missingItems := []string{}
// 	var error error

// 	for key, element := range razeeData {
// 		value, err := RetrieveSecretField(element)
// 		if err != nil {
// 			error = err
// 		}
// 		//TODO: need to think about this.Currently isn't
// 		if value == "" {
// 			missingItems = append(missingItems, key)
// 		}

// 		newField := []byte(fmt.Sprintf(`{"%v": "%v"}`,key,value))
// 		err = json.Unmarshal(newField, &updatedRazeeValues)
// 		if err != nil {
// 			fmt.Println(err)
// 			error = err
// 		}
// 		fmt.Println(updatedRazeeValues)
// 		if err != nil {
// 			error = err
// 		}
// 	}

// 	return updatedRazeeValues,missingItems, error
// }

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

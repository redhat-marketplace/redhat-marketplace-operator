package utils

import (
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

func GetConfigMapNames(configMapList []corev1.ConfigMap)[]string{
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

func ContainsMultiple(inArray []string, referenceArray []string) []string {
	var temp []string 
	for _, searchItem := range referenceArray {
		if !Contains(inArray,searchItem){
			temp = append(temp,searchItem)
		}
		
	}
	return temp
}
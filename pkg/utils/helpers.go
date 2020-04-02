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

// Contains() checks if the lsit contains the key, if so - return it
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}

// Remove() will remove the key from the list
func Remove(list []string, key string) []string {
	for i, s := range list {
		if s == key {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

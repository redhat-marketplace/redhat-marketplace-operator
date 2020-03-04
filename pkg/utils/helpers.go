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

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
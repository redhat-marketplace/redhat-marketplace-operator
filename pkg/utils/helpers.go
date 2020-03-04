package utils
import (
	corev1 "k8s.io/api/core/v1"
	batch "k8s.io/api/batch/v1"
)

func GetNamespaceNames(ns []corev1.Namespace) []string {
	var namespaceNames []string
	for _, namespace := range ns {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	return namespaceNames
}

func GetJobNames(j []batch.Job) []string {
	var jobNames []string
	for _, job := range j {
		jobNames = append(jobNames, job.Name)
	}

	return jobNames
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
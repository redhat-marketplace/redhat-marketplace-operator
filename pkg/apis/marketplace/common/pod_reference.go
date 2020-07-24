package common

import corev1 "k8s.io/api/core/v1"

type PodReference struct {
	// Namespace of the job
	// Required
	Namespace string `json:"namespace"`

	// Name of the job
	// Required
	Name string `json:"name"`
}

func (p *PodReference) FromPod(pod *corev1.Pod) {
	p.Namespace = pod.Namespace
	p.Name = pod.Name
}

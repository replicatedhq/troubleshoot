package types

import (
	corev1 "k8s.io/api/core/v1"
)

type PodDetails struct {
	PodDefinition corev1.Pod     `json:"podDefinition"`
	PodEvents     []corev1.Event `json:"podEvents"`
	PodContainers []PodContainer `json:"podContainers"`
}

type PodContainer struct {
	Name            string `json:"name"`
	LogsFilePath    string `json:"logsFilePath"`
	IsInitContainer bool   `json:"isInitContainer"`
}

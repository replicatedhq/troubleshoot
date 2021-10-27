package k8sutil

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type PodStatusReason string

const (
	PodStatusReasonRunning              PodStatusReason = "Running"
	PodStatusReasonError                PodStatusReason = "Error"
	PodStatusReasonNotReady             PodStatusReason = "NotReady"
	PodStatusReasonUnknown              PodStatusReason = "Unknown"
	PodStatusReasonShutdown             PodStatusReason = "Shutdown"
	PodStatusReasonTerminating          PodStatusReason = "Terminating"
	PodStatusReasonCrashLoopBackOff     PodStatusReason = "CrashLoopBackOff"
	PodStatusReasonImagePullBackOff     PodStatusReason = "ImagePullBackOff"
	PodStatusReasonContainerCreating    PodStatusReason = "ContainerCreating"
	PodStatusReasonPending              PodStatusReason = "Pending"
	PodStatusReasonCompleted            PodStatusReason = "Completed"
	PodStatusReasonEvicted              PodStatusReason = "Evicted"
	PodStatusReasonInitError            PodStatusReason = "Init:Error"
	PodStatusReasonInitCrashLoopBackOff PodStatusReason = "Init:CrashLoopBackOff"
)

// reference: https://github.com/kubernetes/kubernetes/blob/e8fcd0de98d50f4019561a6b7a0287f5c059267a/pkg/printers/internalversion/printers.go#L741
func GetPodStatusReason(pod *corev1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			if hasPodReadyCondition(pod.Status.Conditions) {
				reason = "Running"
			} else {
				reason = "NotReady"
			}
		}
	}

	// "NodeLost" is originally k8s.io/kubernetes/pkg/util/node.NodeUnreachablePodReason but didn't wanna import all of kubernetes package just for this type
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason
}

func hasPodReadyCondition(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsPodUnhealthy(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
		return true
	}

	reason := GetPodStatusReason(pod)

	switch PodStatusReason(reason) {
	case PodStatusReasonRunning:
	case PodStatusReasonCompleted:
		return false
	}

	return true
}

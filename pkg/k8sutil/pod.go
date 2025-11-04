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

// isNativeSidecar checks if an init container is a native sidecar.
// Native sidecars are init containers with restartPolicy: Always (Kubernetes 1.28+).
// They run continuously alongside main containers, unlike traditional init containers
// which must complete before main containers start.
func isNativeSidecar(pod *corev1.Pod, initContainerIndex int) bool {
	// Bounds check - ensure the index is valid
	if initContainerIndex >= len(pod.Spec.InitContainers) {
		return false
	}

	initContainer := pod.Spec.InitContainers[initContainerIndex]

	// Check if RestartPolicy is set to Always
	if initContainer.RestartPolicy != nil && *initContainer.RestartPolicy == corev1.ContainerRestartPolicyAlways {
		return true
	}

	return false
}

// reference: https://github.com/kubernetes/kubernetes/blob/e8fcd0de98d50f4019561a6b7a0287f5c059267a/pkg/printers/internalversion/printers.go#L741
func GetPodStatusReason(pod *corev1.Pod) (string, string) {
	reason := string(pod.Status.Phase)
	// message is used to store more detailed information about the pod status
	message := ""
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
		case isNativeSidecar(pod, i) && container.State.Running != nil:
			// Native sidecar running - this is expected, not stuck initializing.
			// Native sidecars (init containers with restartPolicy: Always) are designed
			// to run continuously, so a Running state means successful initialization.
			continue
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}

		if container.LastTerminationState.Terminated != nil && container.LastTerminationState.Terminated.Message != "" {
			message += container.LastTerminationState.Terminated.Message
		}
		break
	}
	if !initializing {
		hasRunning := false

		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
				if container.LastTerminationState.Terminated != nil {
					// if the container is terminated, we should use the message from the last termination state
					// if no message from the last termination state, we should use the exit code
					if container.LastTerminationState.Terminated.Message != "" {
						message += container.LastTerminationState.Terminated.Message
					} else {
						message += fmt.Sprintf("ExitCode:%d", container.LastTerminationState.Terminated.ExitCode)
					}
				}
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
				// add message from the last termination exit code
				message += fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				// no extra message from the last termination state, since the signal or exit code is used as the reason
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

		// if the pod is not running, check if there is any pod condition reporting as "False" status
		if len(pod.Status.Conditions) > 0 {
			for condition := range pod.Status.Conditions {
				if pod.Status.Conditions[condition].Type == corev1.PodScheduled && pod.Status.Conditions[condition].Status == corev1.ConditionFalse {
					message += pod.Status.Conditions[condition].Message
				}
			}
		}
	}

	// "NodeLost" is originally k8s.io/kubernetes/pkg/util/node.NodeUnreachablePodReason but didn't wanna import all of kubernetes package just for this type
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason, message
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

	reason, _ := GetPodStatusReason(pod)
	if PodStatusReason(reason) == PodStatusReasonCompleted {
		return false // completed pods are healthy pods
	}

	if PodStatusReason(reason) != PodStatusReasonRunning {
		return true // pods that are not completed or running are unhealthy
	}

	// running pods with unready containers are not healthy
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return true
		}
	}

	return false
}

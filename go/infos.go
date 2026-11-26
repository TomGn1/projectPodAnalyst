package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/kubernetes"
)

func getContainerStatus(pod *corev1.Pod, containerName string) (*corev1.ContainerStatus, error) {
	switch len(pod.Status.ContainerStatuses) {
	case 0:
		return nil, errContainerNotReady
	case 1:
		if containerName == "" {
			return &pod.Status.ContainerStatuses[0], nil
		}
	default:
		if containerName == "" {
			return nil, fmt.Errorf("pod %q has multiple containers, please specify --container", pod.Name)
		}
	}

	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == containerName {
			return &pod.Status.ContainerStatuses[i], nil
		}
	}
	return nil, fmt.Errorf("container %q not found in pod %q", containerName, pod.Name)
}

func returnPodInfos(ctx context.Context, targetPod, targetNamespace, containerName string, clientset *kubernetes.Clientset) (nodeName string, containerID string, err error) {
	// Get specified pod in the specified namespace

	p, err := clientset.CoreV1().Pods(targetNamespace).Get(ctx, targetPod, metav1.GetOptions{})

	if err != nil {
		switch {
		case apierrors.IsNotFound(err):
			return "", "", fmt.Errorf("pod %q not found in namespace %q: %w", targetPod, targetNamespace, err)

		case apierrors.IsForbidden(err):
			return "", "", fmt.Errorf("access denied to pod %q in namespace %q: %w", targetPod, targetNamespace, err)

		default:
			return "", "", fmt.Errorf("failed to get pod %q: %w", targetPod, err)
		}
	}

	if p.Spec.NodeName == "" {
		return "", "", errPodNotScheduled
	}

	containerStatus, err := getContainerStatus(p, containerName)
	if err != nil {
		return "", "", err
	}

	if containerStatus.ContainerID == "" {
		return "", "", errContainerNotReady
	}

	return p.Spec.NodeName, containerStatus.ContainerID, nil
}

func watchPod(ctx context.Context, analystPodName string, clientset *kubernetes.Clientset) error {

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)

	defer cancel()

	watcher, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).Watch(ctx, metav1.ListOptions{FieldSelector: "metadata.name=" + analystPodName})

	if err != nil {
		return err
	}

	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, channelOpen := <-watcher.ResultChan():
			if !channelOpen {
				if err := ctx.Err(); err != nil {
					return err
				}

				return errors.New("watch channel closed")
			}

			if event.Type == watch.Error {
				err := apierrors.FromObject(event.Object)
				return fmt.Errorf("watch error: %w", err)
			}

			podSt, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			fmt.Println("EVENT:", event.Type, "PHASE:", podSt.Status.Phase)
			// Watch pod phase
			switch podSt.Status.Phase {

			case corev1.PodSucceeded:
				return nil

			case corev1.PodPending:
				if len(podSt.Status.ContainerStatuses) == 0 {
					continue
				}

				waiting := podSt.Status.ContainerStatuses[0].State.Waiting
				if waiting == nil {
					continue
				}

				switch waiting.Reason {
				case "ImagePullBackOff", "ErrImagePull", "CreateContainerError", "CreateContainerConfigError":
					return fmt.Errorf("analyst pod %s stuck, reason: %s", analystPodName, waiting.Reason)
				default:
					continue

				}

			case corev1.PodFailed:
				if len(podSt.Status.ContainerStatuses) == 0 {
					return errors.New("pod failed: no container status available")
				}

				state := podSt.Status.ContainerStatuses[0].State
				if state.Terminated == nil {
					return errors.New("pod failed: terminated state not available")
				}

				term := state.Terminated
				return fmt.Errorf("pod failed: %s, exit code %d, message: %s",
					term.Reason,
					term.ExitCode,
					term.Message,
				)
			}
		}
	}
}

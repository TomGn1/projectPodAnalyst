package main

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// Create a pod with volumes and configmap mounted, running crictl inspect command.
func createPod(ctx context.Context, clientset *kubernetes.Clientset, nodeName string, containerID string, confName string) (analystPodName string, err error) {
	podClient := clientset.CoreV1().Pods(corev1.NamespaceDefault)
	hostPathType := corev1.HostPathFile
	socketType := corev1.HostPathSocket
	podManifest := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pod-analyst-",
		},
		Spec: corev1.PodSpec{
			HostPID:       false,
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:                     "crictl",
					Image:                    "busybox",
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "/usr/bin/crictl", Name: "crictl"},
						{
							MountPath: "/run/containerd/containerd.sock",
							Name:      "containerd-socket",
						},
						{
							MountPath: "/etc/crictl.yaml",
							SubPath:   "crictl.yaml",
							Name:      "config",
							ReadOnly:  true,
						},
					},
					Command: []string{
						"/usr/bin/crictl",
					},
					Args: []string{
						"inspect",
						containerID,
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "crictl",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/usr/bin/crictl",
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "containerd-socket",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/run/containerd/containerd.sock",
							Type: &socketType,
						},
					},
				},
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: confName},
							Items:                []corev1.KeyToPath{{Key: "crictl.yaml", Path: "crictl.yaml"}},
						},
					},
				},
			},
		},
	}

	fmt.Println("Creating pod...")
	result, err := podClient.Create(ctx, podManifest, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return result.Name, nil
}

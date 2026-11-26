package main

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// Create a configmap to configure crictl
func createConfigmap(ctx context.Context, clientset *kubernetes.Clientset) (*corev1.ConfigMap, error) {
	configMapClient := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crictl-config",
		},
		Data: map[string]string{
			"crictl.yaml": `runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 2
debug: false
`,
		},
	}

	fmt.Println("Creating configmap...")
	result, err := configMapClient.Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

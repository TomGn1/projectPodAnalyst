package main

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

func deleteConfigmap(ctx context.Context, configMapName string, clientset *kubernetes.Clientset) error {

	childCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	gracePeriodSeconds := int64(0)

	err := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Delete(childCtx, configMapName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	})

	return err
}

func deletePod(ctx context.Context, analystPodName string, clientset *kubernetes.Clientset) error {

	childCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	gracePeriodSeconds := int64(0)

	err := clientset.CoreV1().Pods(corev1.NamespaceDefault).Delete(childCtx, analystPodName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	})

	return err
}

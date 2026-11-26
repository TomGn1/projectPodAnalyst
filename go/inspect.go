package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
)

type crictlInspect struct {
	Info struct {
		PID *int `json:"pid"`
	} `json:"info"`
}

func getPid(ctx context.Context, analystPodName string, clientset *kubernetes.Clientset) (int, error) {

	req := clientset.CoreV1().Pods(corev1.NamespaceDefault).GetLogs(analystPodName, &corev1.PodLogOptions{})

	stream, err := req.Stream(ctx)
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := stream.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	data, err := io.ReadAll(stream)
	if err != nil {
		return 0, err
	}

	var inspect crictlInspect

	if err = json.Unmarshal(data, &inspect); err != nil {
		return 0, err
	}

	if inspect.Info.PID == nil {
		return 0, errors.New("pid not found in inspect output")
	}

	return *inspect.Info.PID, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
	)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// The goal of this function is to get the PID of the targeted container, directrly from the host.
func run(ctx context.Context) error {

	// Parsing argument with the following flags : --pod, --namespace, --container
	targetPod, targetNamespace, containerName, err := parserFlags()
	if err != nil {
		return err
	}

	clientset, err := CreateKubernetesClient()
	if err != nil {
		return err
	}

	// Get the node name and the container ID of the targeted pod
	var nodeName, containerID string

	targetPodCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var lastErr error

	for {
		nodeName, containerID, err = returnPodInfos(targetPodCtx, targetPod, targetNamespace, containerName, clientset)
		if err == nil {
			break
		}

		if !errors.Is(err, lastErr) {
			switch {
			case errors.Is(err, errPodNotScheduled):
				fmt.Println("pod not scheduled...")

			case errors.Is(err, errContainerNotReady):
				fmt.Println("waiting for target pod to become ready...")

			default:
				return err
			}

			lastErr = err
		}

		select {
		case <-time.After(2 * time.Second):
		case <-targetPodCtx.Done():
			return fmt.Errorf("timed out waiting for target pod: %w", targetPodCtx.Err())
		}
	}

	fmt.Printf("Node: %s\nContainerID: %s\n", nodeName, containerID)

	parts := strings.SplitN(containerID, "://", 2)
	if len(parts) == 2 {
		containerID = parts[1]
	}

	// Emulate kubectl run command with a pod runing crictl inspect and a configmap configuring crictl
	configMap, err := createConfigmap(ctx, clientset)
	if err != nil {
		return err
	}

	defer func() {
		if err := deleteConfigmap(ctx, configMap.Name, clientset); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	analystPodName, err := createPod(ctx, clientset, nodeName, containerID, configMap.Name)
	if err != nil {
		return err
	}

	defer func() {
		if err := deletePod(ctx, analystPodName, clientset); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	// Watch the phase and status of the created pod
	if err = watchPod(ctx, analystPodName, clientset); err != nil {
		return err
	}

	// Get and decode the PID from the displayed json
	pid, err := getPid(ctx, analystPodName, clientset)
	if err != nil {
		return err
	}

	fmt.Printf("PID: %d\n", pid)

	return nil
}

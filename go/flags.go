package main

import (
	"errors"
	"flag"
)

func parserFlags() (string, string, string, error) {
	targetNamespace := flag.String("namespace", "default", "Kubernetes Namespace")
	targetPod := flag.String("pod", "", "Pod Name")
	containerName := flag.String("container", "", "Container Name")
	flag.Parse()

	if *targetPod == "" {
		flag.Usage()
		return "", "", "", errors.New("pod name required")
	}

	return *targetPod, *targetNamespace, *containerName, nil
}

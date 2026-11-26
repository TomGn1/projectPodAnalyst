package main

import (
	"errors"
)

var (
	errContainerNotReady = errors.New("container not ready")
	errPodNotScheduled   = errors.New("pod not scheduled")
)

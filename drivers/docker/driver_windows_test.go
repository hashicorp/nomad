// +build windows

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containing nanoserver windows and
	// a busybox exe.
	busyboxImageID := "stefanscherer/busybox-windows@sha256:af396324c4c62e369a388ebb38d4efd44211dc7c95a438e6feb62b4ae4194c5b"

	return TaskConfig{
		Image:   busyboxImageID,
		Command: command[0],
		Args:    command[1:],
	}
}

// No-op on windows because we don't load images.
func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
}

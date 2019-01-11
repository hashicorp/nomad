// +build windows

package docker

import (
	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/nomad/client/allocdir"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containting nanoserver windows and
	// a busybox exe.
	// See https://github.com/dantoml/windows/blob/81cff1ed77729d1fa36721abd6cb6efebff2f8ef/docker/busybox/Dockerfile
	busyboxImageID := "dantoml/busybox-windows:08012019"

	return TaskConfig{
		Image:   busyboxImageID,
		Command: command[0],
		Args:    command[1:],
	}
}

func copyImage(t testing.T, taskDir *allocdir.TaskDir, image string) {
}

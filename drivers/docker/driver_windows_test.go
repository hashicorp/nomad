//go:build windows
// +build windows

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	tu "github.com/hashicorp/nomad/testutil"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containing nanoserver windows and
	// a busybox exe.
	busyboxImageID := "hashicorpnomad/busybox-windows:server2016-0.1"

	if tu.IsCI() {
		// In CI, use HashiCorp Mirror to avoid DockerHub rate limiting
		busyboxImageID = "docker.mirror.hashicorp.services/" + busyboxImageID
	}

	return TaskConfig{
		Image:            busyboxImageID,
		ImagePullTimeout: "5m",
		Command:          command[0],
		Args:             command[1:],
	}
}

// No-op on windows because we don't load images.
func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
}

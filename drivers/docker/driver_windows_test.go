// +build windows

package docker

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containing nanoserver windows and
	// a busybox exe.
	// See https://github.com/dantoml/windows/blob/81cff1ed77729d1fa36721abd6cb6efebff2f8ef/docker/busybox/Dockerfile
	busyboxImageID := "dantoml/busybox-windows:08012019"

	return TaskConfig{
		Image:   busyboxImageID,
		Command: command[0],
		Args:    command[1:],
	}
}

// No-op on windows because we don't load images.
func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
}

func TestDockerDriver_Windows_Volumes(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	taskCfg.Volumes = []string{
		// absolute mounting
		`C:\Windows:c:\TempWindows`,

		// relative path from local
		`local:c:\relativelocalpath`,

		// Pipes
		`\\.\pipe\docker_engine:\\.\pipe\docker_engine`,
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	binds := make([]string, len(container.HostConfig.Binds))
	for i, v := range container.HostConfig.Binds {
		binds[i] = strings.ToLower(v)
	}

	allocDir := strings.ToLower(task.AllocDir)
	require.Contains(t, binds, allocDir+`\nc-demo\local:c:\local`, "implicit volume")
	require.Contains(t, binds, `c:\windows:c:\tempwindows`, "absolute path")
	require.Contains(t, binds, allocDir+`\nc-demo\local:c:\relativelocalpath`, "relative volume")
	require.Contains(t, binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`, "pipes")
}

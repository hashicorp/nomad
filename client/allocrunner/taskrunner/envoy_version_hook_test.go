package taskrunner

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var (
	taskEnvDefault = taskenv.NewTaskEnv(nil, nil, nil, map[string]string{
		"meta.connect.sidecar_image": envoy.ImageFormat,
		"meta.connect.gateway_image": envoy.ImageFormat,
	}, "", "")
)

func TestEnvoyVersionHook_semver(t *testing.T) {
	t.Parallel()

	t.Run("with v", func(t *testing.T) {
		result, err := semver("v1.2.3")
		require.NoError(t, err)
		require.Equal(t, "1.2.3", result)
	})

	t.Run("without v", func(t *testing.T) {
		result, err := semver("1.2.3")
		require.NoError(t, err)
		require.Equal(t, "1.2.3", result)
	})

	t.Run("unexpected", func(t *testing.T) {
		_, err := semver("foo")
		require.EqualError(t, err, "unexpected envoy version format: Malformed version: foo")
	})
}

func TestEnvoyVersionHook_taskImage(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			// empty
		})
		require.Equal(t, envoy.ImageFormat, result)
	})

	t.Run("not a string", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			"image": 7, // not a string
		})
		require.Equal(t, envoy.ImageFormat, result)
	})

	t.Run("normal", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			"image": "custom/envoy:latest",
		})
		require.Equal(t, "custom/envoy:latest", result)
	})
}

func TestEnvoyVersionHook_tweakImage(t *testing.T) {
	t.Parallel()

	image := envoy.ImageFormat

	t.Run("legacy", func(t *testing.T) {
		result, err := (*envoyVersionHook)(nil).tweakImage(image, nil)
		require.NoError(t, err)
		require.Equal(t, envoy.FallbackImage, result)
	})

	t.Run("unexpected", func(t *testing.T) {
		_, err := (*envoyVersionHook)(nil).tweakImage(image, map[string][]string{
			"envoy": {"foo", "bar", "baz"},
		})
		require.EqualError(t, err, "unexpected envoy version format: Malformed version: foo")
	})

	t.Run("standard envoy", func(t *testing.T) {
		result, err := (*envoyVersionHook)(nil).tweakImage(image, map[string][]string{
			"envoy": {"1.15.0", "1.14.4", "1.13.4", "1.12.6"},
		})
		require.NoError(t, err)
		require.Equal(t, "envoyproxy/envoy:v1.15.0", result)
	})

	t.Run("custom image", func(t *testing.T) {
		custom := "custom-${NOMAD_envoy_version}/envoy:${NOMAD_envoy_version}"
		result, err := (*envoyVersionHook)(nil).tweakImage(custom, map[string][]string{
			"envoy": {"1.15.0", "1.14.4", "1.13.4", "1.12.6"},
		})
		require.NoError(t, err)
		require.Equal(t, "custom-1.15.0/envoy:1.15.0", result)
	})
}

func TestEnvoyVersionHook_interpolateImage(t *testing.T) {
	t.Parallel()

	hook := (*envoyVersionHook)(nil)

	t.Run("default sidecar", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": envoy.SidecarConfigVar},
		}
		hook.interpolateImage(task, taskEnvDefault)
		require.Equal(t, envoy.ImageFormat, task.Config["image"])
	})

	t.Run("default gateway", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": envoy.GatewayConfigVar},
		}
		hook.interpolateImage(task, taskEnvDefault)
		require.Equal(t, envoy.ImageFormat, task.Config["image"])
	})

	t.Run("custom static", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": "custom/envoy"},
		}
		hook.interpolateImage(task, taskEnvDefault)
		require.Equal(t, "custom/envoy", task.Config["image"])
	})

	t.Run("custom interpolated", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": "${MY_ENVOY}"},
		}
		hook.interpolateImage(task, taskenv.NewTaskEnv(map[string]string{
			"MY_ENVOY": "my/envoy",
		}, map[string]string{
			"MY_ENVOY": "my/envoy",
		}, nil, nil, "", ""))
		require.Equal(t, "my/envoy", task.Config["image"])
	})

	t.Run("no image", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{},
		}
		hook.interpolateImage(task, taskEnvDefault)
		require.Empty(t, task.Config)
	})
}

func TestEnvoyVersionHook_skip(t *testing.T) {
	t.Parallel()

	h := new(envoyVersionHook)

	t.Run("not docker", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "exec",
				Config: nil,
			},
		})
		require.True(t, skip)
	})

	t.Run("not connect", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "docker",
				Kind:   "",
			},
		})
		require.True(t, skip)
	})

	t.Run("version not needed", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "docker",
				Kind:   structs.NewTaskKind(structs.ConnectProxyPrefix, "task"),
				Config: map[string]interface{}{
					"image": "custom/envoy:latest",
				},
			},
		})
		require.True(t, skip)
	})

	t.Run("version needed custom", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "docker",
				Kind:   structs.NewTaskKind(structs.ConnectProxyPrefix, "task"),
				Config: map[string]interface{}{
					"image": "custom/envoy:v${NOMAD_envoy_version}",
				},
			},
		})
		require.False(t, skip)
	})

	t.Run("version needed standard", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "docker",
				Kind:   structs.NewTaskKind(structs.ConnectProxyPrefix, "task"),
				Config: map[string]interface{}{
					"image": envoy.ImageFormat,
				},
			},
		})
		require.False(t, skip)
	})
}

func TestTaskRunner_EnvoyVersionHook_Prestart_standard(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook")
	defer cleanupDir()

	// Setup a mock for Consul API
	spAPI := consul.MockSupportedProxiesAPI{
		Value: map[string][]string{
			"envoy": {"1.15.0", "1.14.4"},
		},
		Error: nil,
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, spAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert the Task.Config[image] is concrete
	require.Equal(t, "envoyproxy/envoy:v1.15.0", request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_custom(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	alloc.Job.TaskGroups[0].Tasks[0].Config["image"] = "custom-${NOMAD_envoy_version}:latest"
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook")
	defer cleanupDir()

	// Setup a mock for Consul API
	spAPI := consul.MockSupportedProxiesAPI{
		Value: map[string][]string{
			"envoy": {"1.14.1", "1.13.3"},
		},
		Error: nil,
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, spAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert the Task.Config[image] is concrete
	require.Equal(t, "custom-1.14.1:latest", request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_skip(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "exec"
	alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"command": "/sidecar",
	}
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook")
	defer cleanupDir()

	// Setup a mock for Consul API
	spAPI := consul.MockSupportedProxiesAPI{
		Value: map[string][]string{
			"envoy": {"1.14.1", "1.13.3"},
		},
		Error: nil,
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, spAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert the Task.Config[image] does not get set
	require.Empty(t, request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_fallback(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook")
	defer cleanupDir()

	// Setup a mock for Consul API
	spAPI := consul.MockSupportedProxiesAPI{
		Value: nil, // old consul, no .xDS.SupportedProxies
		Error: nil,
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, spAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert the Task.Config[image] is the fallback image
	require.Equal(t, "envoyproxy/envoy:v1.11.2@sha256:a7769160c9c1a55bb8d07a3b71ce5d64f72b1f665f10d81aa1581bc3cf850d09", request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_error(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook")
	defer cleanupDir()

	// Setup a mock for Consul API
	spAPI := consul.MockSupportedProxiesAPI{
		Value: nil,
		Error: errors.New("some consul error"),
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, spAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook, error should be recoverable
	err := h.Prestart(context.Background(), request, &response)
	require.EqualError(t, err, "error retrieving supported Envoy versions from Consul: some consul error")

	// Assert the hook is not Done
	require.False(t, response.Done)
}

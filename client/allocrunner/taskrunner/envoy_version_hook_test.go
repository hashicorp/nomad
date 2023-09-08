// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var (
	taskEnvDefault = taskenv.NewTaskEnv(nil, nil, nil, map[string]string{
		"meta.connect.sidecar_image": envoy.ImageFormat,
		"meta.connect.gateway_image": envoy.ImageFormat,
	}, "", "")
)

func TestEnvoyVersionHook_semver(t *testing.T) {
	ci.Parallel(t)

	t.Run("with v", func(t *testing.T) {
		result, err := semver("v1.2.3")
		must.NoError(t, err)
		must.Eq(t, "1.2.3", result)
	})

	t.Run("without v", func(t *testing.T) {
		result, err := semver("1.2.3")
		must.NoError(t, err)
		must.Eq(t, "1.2.3", result)
	})

	t.Run("unexpected", func(t *testing.T) {
		_, err := semver("foo")
		must.ErrorContains(t, err, "unexpected envoy version format: Malformed version: foo")
	})
}

func TestEnvoyVersionHook_taskImage(t *testing.T) {
	ci.Parallel(t)

	t.Run("absent", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			// empty
		})
		must.Eq(t, envoy.ImageFormat, result)
	})

	t.Run("not a string", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			"image": 7, // not a string
		})
		must.Eq(t, envoy.ImageFormat, result)
	})

	t.Run("normal", func(t *testing.T) {
		result := (*envoyVersionHook)(nil).taskImage(map[string]interface{}{
			"image": "custom/envoy:latest",
		})
		must.Eq(t, "custom/envoy:latest", result)
	})
}

func TestEnvoyVersionHook_tweakImage(t *testing.T) {
	ci.Parallel(t)

	image := envoy.ImageFormat

	t.Run("legacy", func(t *testing.T) {
		_, err := (*envoyVersionHook)(nil).tweakImage(image, nil)
		must.Error(t, err)
	})

	t.Run("unexpected", func(t *testing.T) {
		_, err := (*envoyVersionHook)(nil).tweakImage(image, map[string][]string{
			"envoy": {"foo", "bar", "baz"},
		})
		must.ErrorContains(t, err, "unexpected envoy version format: Malformed version: foo")
	})

	t.Run("standard envoy", func(t *testing.T) {
		result, err := (*envoyVersionHook)(nil).tweakImage(image, map[string][]string{
			"envoy": {"1.15.0", "1.14.4", "1.13.4", "1.12.6"},
		})
		must.NoError(t, err)
		must.Eq(t, "docker.io/envoyproxy/envoy:v1.15.0", result)
	})

	t.Run("custom image", func(t *testing.T) {
		custom := "custom-${NOMAD_envoy_version}/envoy:${NOMAD_envoy_version}"
		result, err := (*envoyVersionHook)(nil).tweakImage(custom, map[string][]string{
			"envoy": {"1.15.0", "1.14.4", "1.13.4", "1.12.6"},
		})
		must.NoError(t, err)
		must.Eq(t, "custom-1.15.0/envoy:1.15.0", result)
	})
}

func TestEnvoyVersionHook_interpolateImage(t *testing.T) {
	ci.Parallel(t)

	hook := (*envoyVersionHook)(nil)

	t.Run("default sidecar", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": envoy.SidecarConfigVar},
		}
		hook.interpolateImage(task, taskEnvDefault)
		must.Eq(t, envoy.ImageFormat, task.Config["image"])
	})

	t.Run("default gateway", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": envoy.GatewayConfigVar},
		}
		hook.interpolateImage(task, taskEnvDefault)
		must.Eq(t, envoy.ImageFormat, task.Config["image"])
	})

	t.Run("custom static", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{"image": "custom/envoy"},
		}
		hook.interpolateImage(task, taskEnvDefault)
		must.Eq(t, "custom/envoy", task.Config["image"])
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
		must.Eq(t, "my/envoy", task.Config["image"])
	})

	t.Run("no image", func(t *testing.T) {
		task := &structs.Task{
			Config: map[string]interface{}{},
		}
		hook.interpolateImage(task, taskEnvDefault)
		must.MapEmpty(t, task.Config)
	})
}

func TestEnvoyVersionHook_skip(t *testing.T) {
	ci.Parallel(t)

	h := new(envoyVersionHook)

	t.Run("not docker", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "exec",
				Config: nil,
			},
		})
		must.True(t, skip)
	})

	t.Run("not connect", func(t *testing.T) {
		skip := h.skip(&ifs.TaskPrestartRequest{
			Task: &structs.Task{
				Driver: "docker",
				Kind:   "",
			},
		})
		must.True(t, skip)
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
		must.True(t, skip)
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
		must.False(t, skip)
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
		must.False(t, skip)
	})
}

func TestTaskRunner_EnvoyVersionHook_Prestart_standard(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
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
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	must.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the Task.Config[image] is concrete
	must.Eq(t, "docker.io/envoyproxy/envoy:v1.15.0", request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_custom(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "podman"
	alloc.Job.TaskGroups[0].Tasks[0].Config["image"] = "custom-${NOMAD_envoy_version}:latest"
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
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
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	must.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the Task.Config[image] is concrete
	must.Eq(t, "custom-1.14.1:latest", request.Task.Config["image"])
}

func TestTaskRunner_EnvoyVersionHook_Prestart_skip(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "exec"
	alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"command": "/sidecar",
	}
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
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
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	must.NoError(t, h.Prestart(context.Background(), request, &response))

	// Assert the Task.Config[image] does not get set
	must.MapNotContainsKey(t, request.Task.Config, "image")
}

func TestTaskRunner_EnvoyVersionHook_Prestart_no_fallback(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
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
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook
	must.Error(t, h.Prestart(context.Background(), request, &response))
}

func TestTaskRunner_EnvoyVersionHook_Prestart_error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
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
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook, error should be recoverable
	err := h.Prestart(context.Background(), request, &response)
	must.ErrorContains(t, err, "error retrieving supported Envoy versions from Consul: some consul error")
}

func TestTaskRunner_EnvoyVersionHook_Prestart_restart(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
	alloc.Job.TaskGroups[0].Tasks[0] = mock.ConnectSidecarTask()
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyVersionHook", alloc.ID)
	defer cleanupDir()

	// Set up a mock for Consul API.
	mockProxiesAPI := consul.MockSupportedProxiesAPI{
		Value: map[string][]string{
			"envoy": {"1.15.0", "1.14.4"},
		},
		Error: nil,
	}

	// Run envoy_version hook
	h := newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, mockProxiesAPI, logger))

	// Create a prestart request
	request := &ifs.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskEnvDefault,
	}
	must.NoError(t, request.TaskDir.Build(false, nil))

	// Prepare a response
	var response ifs.TaskPrestartResponse

	// Run the hook and ensure the tasks image has been modified.
	must.NoError(t, h.Prestart(context.Background(), request, &response))
	must.Eq(t, "docker.io/envoyproxy/envoy:v1.15.0", request.Task.Config["image"])

	// Overwrite the previously modified image. This is the same behaviour that
	// occurs when the server sends a non-destructive allocation update.
	request.Task.Config["image"] = "${meta.connect.sidecar_image}"

	// Run the Prestart hook function again, and ensure the image is updated.
	must.NoError(t, h.Prestart(context.Background(), request, &response))
	must.Eq(t, "docker.io/envoyproxy/envoy:v1.15.0", request.Task.Config["image"])

	// Run the hook again, and ensure the config is still the same mimicking
	// a non-user initiated restart.
	must.NoError(t, h.Prestart(context.Background(), request, &response))
	must.Eq(t, "docker.io/envoyproxy/envoy:v1.15.0", request.Task.Config["image"])
}

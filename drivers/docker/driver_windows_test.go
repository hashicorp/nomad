// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containing nanoserver windows and
	// a busybox exe.
	busyboxImageID := testutil.TestBusyboxImage()

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

func Test_validateImageUser(t *testing.T) {
	ci.Parallel(t)

	taskCfg := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "busybox-demo",
		User: "nomadUser",
	}
	taskDriverCfg := newTaskConfig("", []string{"sh", "-c", "sleep 1"})

	tests := []struct {
		name          string
		taskUser      string
		containerUser string
		privileged    bool
		isolation     string
		driverConfig  *DriverConfig
		wantErr       bool
		want          string
	}{
		{
			"normal user",
			"nomadUser",
			"nomadUser",
			false,
			"process",
			&DriverConfig{},
			false,
			"",
		},
		{
			"ContainerAdmin image user, non-priviliged",
			"",
			"ContainerAdmin",
			false,
			"process",
			&DriverConfig{},
			true,
			containerAdminErrMsg,
		},
		{
			"ContainerAdmin image user, non-priviliged, but hyper-v",
			"",
			"ContainerAdmin",
			false,
			"hyper-v",
			&DriverConfig{},
			false,
			"",
		},
		{
			"ContainerAdmin task user, non-priviliged",
			"",
			"ContainerAdmin",
			false,
			"process",
			&DriverConfig{},
			true,
			containerAdminErrMsg,
		},
		{
			"ContainerAdmin image user, non-priviliged, but overriden by task user",
			"ContainerUser",
			"ContainerAdmin",
			false,
			"process",
			&DriverConfig{},
			false,
			"",
		},
		{
			"ContainerAdmin image user, non-priviliged, but overriden by windows_allow_insecure_container_admin",
			"ContainerAdmin",
			"ContainerAdmin",
			false,
			"process",
			&DriverConfig{WindowsAllowInsecureContainerAdmin: true},
			false,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskCfg.User = tt.taskUser
			taskDriverCfg.Privileged = tt.privileged
			taskDriverCfg.Isolation = tt.isolation

			err := validateImageUser(tt.containerUser, tt.taskUser, &taskDriverCfg, tt.driverConfig)
			if tt.wantErr {
				must.Error(t, err)
				must.Eq(t, tt.want, containerAdminErrMsg)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

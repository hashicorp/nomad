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

func TestDriver_createImage_validateContainerAdmin(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name      string
		taskCfg   *drivers.TaskConfig
		driverCfg *TaskConfig
		wantErr   bool
		want      string
	}{
		{
			"normal user",
			&drivers.TaskConfig{
				ID:   uuid.Generate(),
				Name: "redis-demo",
				User: "nomadUser",
			},
			&TaskConfig{Privileged: false},
			false,
			"",
		},
		{
			"ContainerAdmin, non-priviliged",
			&drivers.TaskConfig{
				ID:   uuid.Generate(),
				Name: "redis-demo",
				User: "nomadUser",
			},
			&TaskConfig{Privileged: false},
			true,
			containerAdminErrMsg,
		},
		{
			"ContainerAdmin, non-priviliged, but hyper-v",
			&drivers.TaskConfig{
				ID:   uuid.Generate(),
				Name: "redis-demo",
				User: "nomadUser",
			},
			&TaskConfig{Privileged: false, Isolation: "hyper-v"},
			true,
			containerAdminErrMsg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestDockerClient(t)
			dh := dockerDriverHarness(t, nil)
			d := dh.Impl().(*Driver)

			got, err := d.createImage(tt.taskCfg, tt.driverCfg, client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.createImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Driver.createImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

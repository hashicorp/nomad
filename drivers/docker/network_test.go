// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/assert"
)

func TestDriver_createSandboxContainerConfig(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		inputAllocID              string
		inputNetworkCreateRequest *drivers.NetworkCreateRequest
		expectedOutputOpts        *docker.CreateContainerOptions
		name                      string
	}{
		{
			inputAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
			inputNetworkCreateRequest: &drivers.NetworkCreateRequest{
				Hostname: "",
			},
			expectedOutputOpts: &docker.CreateContainerOptions{
				Name: "nomad_init_768b5e8c-a52e-825c-d564-51100230eb62",
				Config: &docker.Config{
					Image: "gcr.io/google_containers/pause-amd64:3.1",
					Labels: map[string]string{
						dockerLabelAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
					},
				},
				HostConfig: &docker.HostConfig{
					NetworkMode:   "none",
					RestartPolicy: docker.RestartUnlessStopped(),
				},
			},
			name: "no input hostname",
		},
		{
			inputAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
			inputNetworkCreateRequest: &drivers.NetworkCreateRequest{
				Hostname: "linux",
			},
			expectedOutputOpts: &docker.CreateContainerOptions{
				Name: "nomad_init_768b5e8c-a52e-825c-d564-51100230eb62",
				Config: &docker.Config{
					Image:    "gcr.io/google_containers/pause-amd64:3.1",
					Hostname: "linux",
					Labels: map[string]string{
						dockerLabelAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
					},
				},
				HostConfig: &docker.HostConfig{
					NetworkMode:   "none",
					RestartPolicy: docker.RestartUnlessStopped(),
				},
			},
			name: "supplied input hostname",
		},
	}

	d := &Driver{
		config: &DriverConfig{
			InfraImage: "gcr.io/google_containers/pause-amd64:3.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := d.createSandboxContainerConfig(tc.inputAllocID, tc.inputNetworkCreateRequest)
			assert.Nil(t, err, tc.name)
			assert.Equal(t, tc.expectedOutputOpts, actualOutput, tc.name)
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
)

func TestDriver_createSandboxContainerConfig(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		inputAllocID              string
		inputNetworkCreateRequest *drivers.NetworkCreateRequest
		expectedOutputOpts        *createContainerOptions
		name                      string
	}{
		{
			inputAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
			inputNetworkCreateRequest: &drivers.NetworkCreateRequest{
				Hostname: "",
			},
			expectedOutputOpts: &createContainerOptions{
				Name: "nomad_init_768b5e8c-a52e-825c-d564-51100230eb62",
				Config: &containerapi.Config{
					Image: "registry.k8s.io/pause-amd64:3.3",
					Labels: map[string]string{
						dockerLabelAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
					},
				},
				Host: &containerapi.HostConfig{
					NetworkMode:   "none",
					RestartPolicy: containerapi.RestartPolicy{Name: containerapi.RestartPolicyUnlessStopped},
				},
			},
			name: "no input hostname",
		},
		{
			inputAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
			inputNetworkCreateRequest: &drivers.NetworkCreateRequest{
				Hostname: "linux",
			},
			expectedOutputOpts: &createContainerOptions{
				Name: "nomad_init_768b5e8c-a52e-825c-d564-51100230eb62",
				Config: &containerapi.Config{
					Image:    "registry.k8s.io/pause-amd64:3.3",
					Hostname: "linux",
					Labels: map[string]string{
						dockerLabelAllocID: "768b5e8c-a52e-825c-d564-51100230eb62",
					},
				},
				Host: &containerapi.HostConfig{
					NetworkMode:   "none",
					RestartPolicy: containerapi.RestartPolicy{Name: containerapi.RestartPolicyUnlessStopped},
				},
			},
			name: "supplied input hostname",
		},
	}

	d := &Driver{
		config: &DriverConfig{
			InfraImage: "registry.k8s.io/pause-amd64:3.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := d.createSandboxContainerConfig(tc.inputAllocID, tc.inputNetworkCreateRequest)
			must.Nil(t, err, must.Sprint(tc.name))
			must.Eq(t, tc.expectedOutputOpts, actualOutput, must.Sprint(tc.name))
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestJobNamespaceConstraintCheckHook_Name(t *testing.T) {
	ci.Parallel(t)

	require.Equal(t, "namespace-constraint-check", new(jobNamespaceConstraintCheckHook).Name())
}

func TestJobNamespaceConstraintCheckHook_taskValidateDriver(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		description string
		driver      string
		ns          *structs.Namespace
		result      bool
	}{
		{
			"No drivers enabled/disabled, allow all",
			"docker",
			&structs.Namespace{},
			true,
		},
		{
			"Only exec and docker are allowed 1/2",
			"docker",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledTaskDrivers: []string{"docker", "exec"}},
			},
			true,
		},
		{
			"Only exec and docker are allowed 2/2",
			"raw_exec",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledTaskDrivers: []string{"docker", "exec"}},
			},
			false,
		},
		{
			"disable takes precedence over enable",
			"docker",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledTaskDrivers:  []string{"docker"},
					DisabledTaskDrivers: []string{"docker"}},
			},
			false,
		},
		{
			"All drivers but docker are allowed 1/2",
			"docker",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					DisabledTaskDrivers: []string{"docker"}},
			},
			false,
		},
		{
			"All drivers but docker are allowed 2/2",
			"raw_exec",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					DisabledTaskDrivers: []string{"docker"}},
			},
			true,
		},
	}

	for _, c := range cases {
		var task = &structs.Task{Driver: c.driver}
		require.Equal(t, c.result, taskValidateDriver(task, c.ns), c.description)
	}
}

func TestJobNamespaceConstraintCheckHook_validate(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a namespace
	ns := mock.Namespace()
	ns.Name = "default" // fix the name
	ns.Capabilities = &structs.NamespaceCapabilities{
		EnabledTaskDrivers:  []string{"docker", "qemu"},
		DisabledTaskDrivers: []string{"exec", "raw_exec"},
	}
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns})

	hook := jobNamespaceConstraintCheckHook{srv: s1}
	job := mock.LifecycleJob()
	job.TaskGroups[0].Tasks[0].Driver = "docker"
	job.TaskGroups[0].Tasks[1].Driver = "qemu"
	job.TaskGroups[0].Tasks[2].Driver = "docker"
	job.TaskGroups[0].Tasks[3].Driver = "qemu"
	_, err := hook.Validate(job)
	require.Nil(t, err)

	job.TaskGroups[0].Tasks[2].Driver = "raw_exec"
	_, err = hook.Validate(job)
	require.Equal(t, err.Error(), "used task driver \"raw_exec\" is not allowed in namespace \"default\"")

	job.TaskGroups[0].Tasks[1].Driver = "exec"
	_, err = hook.Validate(job)
	require.Equal(t, err.Error(), "used task drivers [\"exec\" \"raw_exec\"] are not allowed in namespace \"default\"")
}

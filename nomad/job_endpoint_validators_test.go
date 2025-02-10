// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
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

func TestJobNamespaceConstraintCheckHook_validate_drivers(t *testing.T) {
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

func TestJobNamespaceConstraintCheckHook_taskValidateNetworkMode(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		description string
		mode        string
		ns          *structs.Namespace
		result      bool
	}{
		{
			"No capabilities set, allow all",
			"bridge",
			&structs.Namespace{},
			true,
		},
		{
			"No drivers enabled/disabled, allow all",
			"bridge",
			&structs.Namespace{Capabilities: &structs.NamespaceCapabilities{}},
			true,
		},
		{
			"No mode set and only host allowed",
			"",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledNetworkModes: []string{"host"}},
			},
			true,
		},
		{
			"Only bridge and cni/custom are allowed 1/2",
			"bridge",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledNetworkModes: []string{"bridge", "cni/custom"}},
			},
			true,
		},
		{
			"Only bridge and cni/custom are allowed 2/2",
			"host",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledNetworkModes: []string{"bridge", "cni/custom"}},
			},
			false,
		},
		{
			"disable takes precedence over enable",
			"bridge",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					EnabledNetworkModes:  []string{"bridge"},
					DisabledNetworkModes: []string{"bridge"}},
			},
			false,
		},
		{
			"All modes but host are allowed 1/2",
			"host",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					DisabledNetworkModes: []string{"host"}},
			},
			false,
		},
		{
			"All modes but host are allowed 2/2",
			"bridge",
			&structs.Namespace{
				Capabilities: &structs.NamespaceCapabilities{
					DisabledNetworkModes: []string{"host"}},
			},
			true,
		},
	}

	for _, c := range cases {
		var network = &structs.NetworkResource{Mode: c.mode}
		allowed, _ := taskValidateNetworkMode(network, c.ns)
		must.Eq(t, c.result, allowed, must.Sprint(c.description))
	}
}

func TestJobNamespaceConstraintCheckHook_validate_network_modes(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a namespace
	ns := mock.Namespace()
	ns.Name = "default" // fix the name
	ns.Capabilities = &structs.NamespaceCapabilities{
		EnabledNetworkModes:  []string{"bridge", "cni/allowed"},
		DisabledNetworkModes: []string{"host", "cni/forbidden"},
	}
	must.NoError(t, s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns}))

	hook := jobNamespaceConstraintCheckHook{srv: s1}
	job := mock.LifecycleJob()
	job.TaskGroups[0].Networks = append(job.TaskGroups[0].Networks, &structs.NetworkResource{})
	_, err := hook.Validate(job)
	must.EqError(t, err, "used group network mode \"host\" is not allowed in namespace \"default\"")

	job.TaskGroups[0].Networks[0].Mode = "bridge"
	_, err = hook.Validate(job)
	must.NoError(t, err)

	job.TaskGroups[0].Networks[0].Mode = "host"
	job.TaskGroups[0].Networks = append(job.TaskGroups[0].Networks, &structs.NetworkResource{Mode: "cni/forbidden"})
	_, err = hook.Validate(job)
	must.EqError(t, err, "used group network modes [\"host\" \"cni/forbidden\"] are not allowed in namespace \"default\"")
}

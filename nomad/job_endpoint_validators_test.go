package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestJobNamespaceConstraintCheckHook_Name(t *testing.T) {
	t.Parallel()

	require.Equal(t, "namespace-constraint-check", new(jobNamespaceConstraintCheckHook).Name())
}

func TestJobNamespaceConstraintCheckHook_taskValidateDriver(t *testing.T) {
	t.Parallel()

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
			"disable takes precedence over enable", // TODO: Wanted?
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

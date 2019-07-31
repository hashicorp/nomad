package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/stretchr/testify/require"
)

var mockDrivers = map[string]drivers.DriverPlugin{
	"hostonly": &testutils.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{
				NetIsolationModes: []drivers.NetIsolationMode{drivers.NetIsolationModeHost},
			}, nil
		},
	},
	"group1": &testutils.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{
				NetIsolationModes: []drivers.NetIsolationMode{
					drivers.NetIsolationModeHost, drivers.NetIsolationModeGroup},
			}, nil
		},
	},
	"group2": &testutils.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{
				NetIsolationModes: []drivers.NetIsolationMode{
					drivers.NetIsolationModeHost, drivers.NetIsolationModeGroup},
			}, nil
		},
	},
	"mustinit1": &testutils.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{
				NetIsolationModes: []drivers.NetIsolationMode{
					drivers.NetIsolationModeHost, drivers.NetIsolationModeGroup},
				MustInitiateNetwork: true,
			}, nil
		},
	},
	"mustinit2": &testutils.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{
				NetIsolationModes: []drivers.NetIsolationMode{
					drivers.NetIsolationModeHost, drivers.NetIsolationModeGroup},
				MustInitiateNetwork: true,
			}, nil
		},
	},
}

type mockDriverManager struct {
	pluginmanager.MockPluginManager
}

func (m *mockDriverManager) Dispense(driver string) (drivers.DriverPlugin, error) {
	return mockDrivers[driver], nil
}

func TestNewNetworkManager(t *testing.T) {
	for _, tc := range []struct {
		name        string
		alloc       *structs.Allocation
		err         bool
		mustInit    bool
		errContains string
	}{
		{
			name: "defaults/backwards compat",
			alloc: &structs.Allocation{
				TaskGroup: "group",
				Job: &structs.Job{
					TaskGroups: []*structs.TaskGroup{
						{
							Name:     "group",
							Networks: []*structs.NetworkResource{},
							Tasks: []*structs.Task{
								{
									Name:      "task1",
									Driver:    "group1",
									Resources: &structs.Resources{},
								},
								{
									Name:      "task2",
									Driver:    "group2",
									Resources: &structs.Resources{},
								},
								{
									Name:      "task3",
									Driver:    "mustinit1",
									Resources: &structs.Resources{},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "driver /w must init network",
			alloc: &structs.Allocation{
				TaskGroup: "group",
				Job: &structs.Job{
					TaskGroups: []*structs.TaskGroup{
						{
							Name: "group",
							Networks: []*structs.NetworkResource{
								{
									Mode: "bridge",
								},
							},
							Tasks: []*structs.Task{
								{
									Name:      "task1",
									Driver:    "group1",
									Resources: &structs.Resources{},
								},
								{
									Name:      "task2",
									Driver:    "mustinit2",
									Resources: &structs.Resources{},
								},
							},
						},
					},
				},
			},
			mustInit: true,
		},
		{
			name: "multiple mustinit",
			alloc: &structs.Allocation{
				TaskGroup: "group",
				Job: &structs.Job{
					TaskGroups: []*structs.TaskGroup{
						{
							Name: "group",
							Networks: []*structs.NetworkResource{
								{
									Mode: "bridge",
								},
							},
							Tasks: []*structs.Task{
								{
									Name:      "task1",
									Driver:    "mustinit1",
									Resources: &structs.Resources{},
								},
								{
									Name:      "task2",
									Driver:    "mustinit2",
									Resources: &structs.Resources{},
								},
							},
						},
					},
				},
			},
			err:         true,
			errContains: "want to initiate networking but only one",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			nm, err := newNetworkManager(tc.alloc, &mockDriverManager{})
			if tc.err {
				require.Error(err)
				require.Contains(err.Error(), tc.errContains)
			} else {
				require.NoError(err)
			}

			if tc.mustInit {
				_, ok := nm.(*testutils.MockDriver)
				require.True(ok)
			} else if tc.err {
				require.Nil(nm)
			} else {
				_, ok := nm.(*defaultNetworkManager)
				require.True(ok)
			}
		})
	}

}

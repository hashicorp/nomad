package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsulConnect_Validate(t *testing.T) {
	t.Parallel()

	c := &ConsulConnect{}

	// An empty Connect stanza is invalid
	require.Error(t, c.Validate())

	// Native=true is valid
	c.Native = true
	require.NoError(t, c.Validate())

	// Native=true + Sidecar!=nil is invalid
	c.SidecarService = &ConsulSidecarService{}
	require.Error(t, c.Validate())

	// Native=false + Sidecar!=nil is valid
	c.Native = false
	require.NoError(t, c.Validate())
}

func TestConsulConnect_CopyEquals(t *testing.T) {
	t.Parallel()

	c := &ConsulConnect{
		SidecarService: &ConsulSidecarService{
			Port: "9001",
			Proxy: &ConsulProxy{
				Upstreams: []ConsulUpstream{
					{
						DestinationName: "up1",
						LocalBindPort:   9002,
					},
					{
						DestinationName: "up2",
						LocalBindPort:   9003,
					},
				},
				Config: map[string]interface{}{
					"foo": 1,
				},
			},
		},
	}

	require.NoError(t, c.Validate())

	// Copies should be equivalent
	o := c.Copy()
	require.True(t, c.Equals(o))

	o.SidecarService.Proxy.Upstreams = nil
	require.False(t, c.Equals(o))
}

func TestSidecarTask_MergeIntoTask(t *testing.T) {

	task := MockJob().TaskGroups[0].Tasks[0]
	sTask := &SidecarTask{
		Name:   "sidecar",
		Driver: "sidecar",
		Config: map[string]interface{}{
			"foo": "bar",
		},
		Resources: &Resources{
			CPU:      10000,
			MemoryMB: 10000,
		},
		Env: map[string]string{
			"sidecar": "proxy",
		},
	}

	expected := task.Copy()
	expected.Name = "sidecar"
	expected.Driver = "sidecar"
	expected.Config = map[string]interface{}{
		"foo": "bar",
	}
	expected.Resources.CPU = 10000
	expected.Resources.MemoryMB = 10000
	expected.Env["sidecar"] = "proxy"

	sTask.MergeIntoTask(task)

	require.Exactly(t, expected, task)

	sTask.Config["abc"] = 123
	expected.Config["abc"] = 123

	sTask.MergeIntoTask(task)
	require.Exactly(t, expected, task)

}

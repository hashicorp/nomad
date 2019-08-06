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
				Upstreams: []*ConsulUpstream{
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

package consul

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectProxies_Proxies(t *testing.T) {
	pc := NewConnectProxiesClient(NewMockAgent(ossFeatures))

	proxies, err := pc.Proxies()
	require.NoError(t, err)
	require.Equal(t, map[string][]string{
		"envoy": []string{"1.14.2", "1.13.2", "1.12.4", "1.11.2"},
	}, proxies)
}

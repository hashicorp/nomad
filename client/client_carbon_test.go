package client

import (
	"context"
	"github.com/hashicorp/nomad/client/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestClient_Carbon_CarbonIntensityProvider(t *testing.T) {
	t.Parallel()

	carbonConfig := config.CarbonConfig{
		Region:      "UK",
		ProviderKey: config.CI,
		CarbonIntensityConfig: &config.CarbonIntensityConfig{
			APIUrl: "https://api.carbonintensity.org.uk/intensity",
		},
	}

	err := carbonConfig.Finalize()
	require.NoError(t, err)

	provider := *carbonConfig.Provider
	score, err := provider.GetCarbonIntensity(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, 0, score)
}

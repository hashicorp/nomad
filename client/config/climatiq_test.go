package config

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestClimatiq_AWS_CarbonScore(t *testing.T) {
	t.Skip()
	energyConfig := &EnergyConfig{
		ProviderKey: CQ,
		Region:      "us_east_1",
		ClimatiqConfig: &ClimatiqConfig{
			APIKey: "",
			APIUrl: "https://beta3.api.climatiq.io/compute/{provider}/cpu",
			CloudProviders: []CloudProviderConfig{
				{
					Name:    "aws",
					Regions: []string{"us_east_1", "us_west_2"},
				},
			},
		},
	}
	provider, err := newClimatiqProvider(energyConfig)

	require.NoError(t, err)
	require.NotNil(t, provider)

	err = energyConfig.Finalize()
	require.NoError(t, err)

	score, err := provider.GetCarbonIntensity(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, 0, score)
}

func TestClimatiq_AWS_RecommendRegion(t *testing.T) {
	t.Skip()
	energyConfig := &EnergyConfig{
		ProviderKey: CQ,
		Region:      "us_east_1",
		ClimatiqConfig: &ClimatiqConfig{
			APIKey: "",
			APIUrl: "https://beta3.api.climatiq.io/compute/{provider}/cpu",
			CloudProviders: []CloudProviderConfig{
				{
					Name:    "aws",
					Regions: []string{"us_east_1", "us_west_2"},
				},
			},
		},
	}
	provider, err := newClimatiqProvider(energyConfig)

	require.NoError(t, err)
	require.NotNil(t, provider)

	err = energyConfig.Finalize()
	require.NoError(t, err)

	region, err := provider.RecommendRegion()
	require.NoError(t, err)
	require.NotEqual(t, "", region)
}

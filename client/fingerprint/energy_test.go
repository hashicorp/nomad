package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestEnergyFingerprint(t *testing.T) {
	f := NewEnergyFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{
		EnergyConfig: &config.EnergyConfig{
			ProviderKey: config.CI,
			Region:      "UK",
			CarbonIntensityConfig: &config.CarbonIntensityConfig{
				APIUrl: "https://api.carbonintensity.org.uk/intensity",
			},
		},
	}, Node: node}

	err := request.Config.EnergyConfig.Finalize()
	require.NoError(t, err)

	var response FingerprintResponse
	err = f.Fingerprint(request, &response)

	require.NoError(t, err)
	require.True(t, response.Detected)

	// Energy info
	require.NotNil(t, response.Attributes, "expected attributes to be initialized")
	require.NotEmptyf(t, response.Attributes["energy.carbon_score"], "missing energy.carbon_score")
}

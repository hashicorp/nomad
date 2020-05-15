package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestCNIFingerprint(t *testing.T) {
	cases := []struct {
		name     string
		req      *FingerprintRequest
		exp      *FingerprintResponse
		err      bool
		errMatch string
	}{
		{
			name: "cni config dir not set",
			req: &FingerprintRequest{
				Config: &config.Config{},
			},
			exp: &FingerprintResponse{
				Detected: false,
			},
		},
		{
			name: "cni config dir non-existent",
			req: &FingerprintRequest{
				Config: &config.Config{
					CNIConfigDir: "text_fixtures/cni_nonexistent",
				},
			},
			exp: &FingerprintResponse{
				Detected: false,
			},
		},
		{
			name: "two networks, no errors",
			req: &FingerprintRequest{
				Config: &config.Config{
					CNIConfigDir: "test_fixtures/cni",
				},
			},
			exp: &FingerprintResponse{
				NodeResources: &structs.NodeResources{
					Networks: []*structs.NetworkResource{
						{
							Mode: "cni/net1",
						},
						{
							Mode: "cni/net2",
						},
					},
				},
				Detected: true,
			},
			err: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require := require.New(t)
			fp := NewCNIFingerprint(testlog.HCLogger(t))
			resp := &FingerprintResponse{}
			err := fp.Fingerprint(c.req, resp)
			if c.err {
				require.Error(err)
				require.Contains(err.Error(), c.errMatch)
			} else {
				require.NoError(err)
				require.Exactly(c.exp, resp)
			}
		})
	}
}

package nomad

import (
	"context"
	"math"
	"testing"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_Config(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		name           string
		config         *config.Limits
		expectedTokens uint64
	}{
		{
			name:           "nil config",
			config:         nil,
			expectedTokens: math.MaxUint64,
		},
		{
			name:           "empty config",
			config:         &config.Limits{},
			expectedTokens: math.MaxUint64,
		},
		{
			name: "has write config but no default",
			config: &config.Limits{
				Endpoints: &config.RPCEndpointLimitsSet{
					Namespace: &config.RPCEndpointLimits{
						RPCWriteRate: pointer.Of(100)},
				},
			},
			expectedTokens: uint64(100),
		},
		{
			name: "has default write config but no endpoint config",
			config: &config.Limits{
				RPCDefaultWriteRate: pointer.Of(100)},
			expectedTokens: uint64(100),
		},
		{
			name: "endpoint config overwrites default",
			config: &config.Limits{
				RPCDefaultWriteRate: pointer.Of(100),
				Endpoints: &config.RPCEndpointLimitsSet{
					Namespace: &config.RPCEndpointLimits{
						RPCWriteRate: pointer.Of(200)},
				},
			},
			expectedTokens: uint64(200),
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			limiter := newRateLimiter(ctx, tc.config)
			defer limiter.close()
			limiter.lock.RLock()
			defer limiter.lock.RUnlock()

			nsLimit := limiter.limiters["Namespace"]
			tokens, remaining, err := nsLimit.write.Get(ctx, "unused")
			require.NoError(t, err)
			require.Equal(t, uint64(0), tokens)
			require.Equal(t, uint64(0), remaining)

			tokens, remaining, _, ok, err := nsLimit.write.Take(ctx, "used")
			require.NoError(t, err)
			require.True(t, ok, "did not expect to be rate-limited")
			require.Equal(t, uint64(tc.expectedTokens), tokens)
			require.Equal(t, uint64(tc.expectedTokens-1), remaining)
		})
	}

}

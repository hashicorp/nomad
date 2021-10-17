package taskenv

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func Test_InterpolateNetworks(t *testing.T) {
	testCases := []struct {
		inputTaskEnv           *TaskEnv
		inputNetworks          structs.Networks
		expectedOutputNetworks structs.Networks
		name                   string
	}{
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{Hostname: "my-little-pony"},
			},
			expectedOutputNetworks: structs.Networks{
				{Hostname: "my-little-pony"},
			},
			name: "non-interpolated hostname",
		},
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{Hostname: "${foo}-cache-${baz}"},
			},
			expectedOutputNetworks: structs.Networks{
				{Hostname: "bar-cache-blah"},
			},
			name: "interpolated hostname",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := InterpolateNetworks(tc.inputTaskEnv, tc.inputNetworks)
			assert.Equal(t, tc.expectedOutputNetworks, actualOutput, tc.name)
		})
	}
}

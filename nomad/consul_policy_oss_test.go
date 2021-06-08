//+build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestConsulACLsAPI_hasSufficientPolicy(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, namespace, task string, token *api.ACLToken, exp bool) {
		logger := testlog.HCLogger(t)
		cAPI := &consulACLsAPI{
			aclClient: consul.NewMockACLsAPI(logger),
			logger:    logger,
		}
		result, err := cAPI.canWriteService(namespace, task, token)
		require.NoError(t, err)
		require.Equal(t, exp, result)
	}

	// In Nomad OSS, group consul namespace will always be empty string.

	t.Run("no namespace with default token", func(t *testing.T) {
		t.Run("no useful policy or role", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken0, false)
		})

		t.Run("working policy only", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken1, true)
		})

		t.Run("working role only", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken4, true)
		})
	})
}

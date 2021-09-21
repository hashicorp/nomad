package stream

import (
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestRegisterEventBrokerService(t *testing.T) {
	t.Parallel()

	err := RegisterEventBrokerService(hclog.NewInterceptLogger(hclog.DefaultOptions), os.Stderr, nil)
	require.NoError(t, err)
}

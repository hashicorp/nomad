package testutil

import (
	"net"

	testing "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// RequireDeadlineErr requires that an error be caused by a net.Conn's deadline
// being reached (after being set by conn.Set{Read,Write}Deadline or
// SetDeadline).
func RequireDeadlineErr(t testing.T, err error) {
	t.Helper()

	require.NotNil(t, err)
	netErr, ok := err.(net.Error)
	require.Truef(t, ok, "error does not implement net.Error: (%T) %v", err, err)
	require.Contains(t, netErr.Error(), ": i/o timeout")
	require.True(t, netErr.Timeout())
	require.True(t, netErr.Temporary())
}

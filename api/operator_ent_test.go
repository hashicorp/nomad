//go:build ent
// +build ent

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperator_LicenseGet(t *testing.T) {
	t.Parallel()
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()

	// Make authenticated request.
	_, _, err := operator.LicenseGet(nil)
	require.NoError(t, err)

	// Make unauthenticated request.
	c.SetSecretID("")
	_, _, err = operator.LicenseGet(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
}

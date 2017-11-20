// +build pro

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotas_Errors(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()
	expectedErr := fmt.Errorf("Unexpected response code: 501 (Nomad Premium only endpoint)")

	// Create a quota spec and register it
	qs := testQuotaSpec()
	_, err := quotas.Register(qs, nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	// List quotas
	_, _, err = quotas.List(nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	// List usages
	_, _, err = quotas.ListUsage(nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	_, _, err = quotas.PrefixListUsage("foo", nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())
}

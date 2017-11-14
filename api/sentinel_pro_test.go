// +build pro

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinel_Errors(t *testing.T) {
	t.Parallel()
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.SentinelPolicies()
	expectedErr := fmt.Errorf("Unexpected response code: 501 (Nomad Premium only endpoint)")
	assert := assert.New(t)

	// Listing
	_, _, err := ap.List(nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	// Registering
	policy := &SentinelPolicy{
		Name:             "test",
		Description:      "test",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true }",
	}
	_, err = ap.Upsert(policy, nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	// Delete
	_, err = ap.Delete(policy.Name, nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())

	// Info
	_, _, err = ap.Info(policy.Name, nil)
	assert.NotNil(err)
	assert.Equal(expectedErr.Error(), err.Error())
}

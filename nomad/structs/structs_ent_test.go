// +build ent

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelPolicySetHash(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	out1 := sp.SetHash()
	assert.NotNil(t, out1)
	assert.NotNil(t, sp.Hash)
	assert.Equal(t, out1, sp.Hash)

	sp.Policy = "main = rule { false }"
	out2 := sp.SetHash()
	assert.NotNil(t, out2)
	assert.NotNil(t, sp.Hash)
	assert.Equal(t, out2, sp.Hash)
	assert.NotEqual(t, out1, out2)
}

func TestSentinelPolicy_Validate(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	// Test a good policy
	assert.Nil(t, sp.Validate())

	// Try an invalid name
	sp.Name = "hi@there"
	assert.NotNil(t, sp.Validate())

	// Try an invalid description
	sp.Name = "test"
	sp.Description = string(make([]byte, 1000))
	assert.NotNil(t, sp.Validate())

	// Try an invalid scope
	sp.Description = ""
	sp.Scope = "random"
	assert.NotNil(t, sp.Validate())

	// Try an invalid type
	sp.Scope = SentinelScopeSubmitJob
	sp.EnforcementLevel = "yolo"
	assert.NotNil(t, sp.Validate())

	// Try an invalid policy
	sp.EnforcementLevel = SentinelEnforcementLevelAdvisory
	sp.Policy = "blah 123"
	assert.NotNil(t, sp.Validate())
}

func TestSentinelPolicy_CacheKey(t *testing.T) {
	sp := &SentinelPolicy{
		Name:        "test",
		ModifyIndex: 10,
	}
	assert.Equal(t, "test:10", sp.CacheKey())
}

func TestSentinelPolicy_Compile(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	f, fset, err := sp.Compile()
	assert.Nil(t, err)
	assert.NotNil(t, fset)
	assert.NotNil(t, f)
}

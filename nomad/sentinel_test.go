package nomad

import (
	"testing"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/sentinel/sentinel"
	"github.com/stretchr/testify/assert"
)

func TestServer_SentinelPoliciesByScope(t *testing.T) {
	t.Parallel()
	s1, _ := testACLServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a fake policy
	policy1 := mock.SentinelPolicy()
	policy2 := mock.SentinelPolicy()
	s1.State().UpsertSentinelPolicies(1000,
		[]*structs.SentinelPolicy{policy1, policy2})

	// Ensure we get them back
	ps, err := s1.sentinelPoliciesByScope(structs.SentinelScopeSubmitJob)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(ps))
}

func TestServer_PrepareSentinelPolicies(t *testing.T) {
	t.Parallel()
	s1, _ := testACLServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a fake policy
	policy1 := mock.SentinelPolicy()
	policy2 := mock.SentinelPolicy()
	in := []*structs.SentinelPolicy{policy1, policy2}

	// Test compilation
	prep, err := prepareSentinelPolicies(s1.sentinel, in)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(prep))
}

func TestSentinelResultToWarnErr(t *testing.T) {
	sent := sentinel.New(nil)

	// Setup three policies:
	// p1: Fails, Hard-mandatory (err)
	// p2: Fails, Soft-mandatory + Override (warn)
	// p3: Fails, Advisory (warn)
	p1 := mock.SentinelPolicy()
	p1.Type = structs.SentinelTypeHardMandatory
	p1.Policy = "main = rule { false }"

	p2 := mock.SentinelPolicy()
	p2.Type = structs.SentinelTypeSoftMandatory
	p2.Policy = "main = rule { false }"

	p3 := mock.SentinelPolicy()
	p3.Type = structs.SentinelTypeAdvisory
	p3.Policy = "main = rule { false }"

	// Prepare the policies
	ps := []*structs.SentinelPolicy{p1, p2, p3}
	prep, err := prepareSentinelPolicies(sent, ps)
	assert.Nil(t, err)

	// Evaluate with an override
	result := sent.Eval(prep, &sentinel.EvalOpts{
		Override: true,
		EvalAll:  true, // For testing
	})

	// Get the errors
	warn, err := sentinelResultToWarnErr(result)
	assert.NotNil(t, err)
	assert.NotNil(t, warn)

	// Check the lengths
	assert.Equal(t, 1, len(err.(*multierror.Error).Errors))
	assert.Equal(t, 2, len(warn.(*multierror.Error).Errors))
}

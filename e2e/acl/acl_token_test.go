package acl

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

var (
	minTokenExpiryDur = 1 * time.Second
	maxTokenExpiryDur = 24 * time.Hour
)

// testACLTokenExpiration tests tokens that have expiration values set.
// Expirations are timing based which makes this test sensitive to timing
// problems when running the E2E suite.
//
// When running the test, the Nomad server ACL config must have the
// token_min_expiration_ttl value set to minTokenExpiryDur. The
// token_min_expiration_ttl value must be set to maxTokenExpiryDur if this
// value differs from the default. This is so we can test expired tokens,
// without blocking for an extended period of time as well as other time
// related aspects.
func testACLTokenExpiration(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Create and defer the cleanup process. This is used to remove all
	// resources created by this test and covers situations where the test
	// fails or during normal running.
	cleanUpProcess := newCleanup()
	defer cleanUpProcess.run(t, nomadClient)

	// Create an ACL policy which will be assigned to the created ACL tokens.
	customNamespacePolicy := api.ACLPolicy{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
		Rules:       `namespace "default" {policy = "read"}`,
	}
	_, err := nomadClient.ACLPolicies().Upsert(&customNamespacePolicy, nil)
	require.NoError(t, err)

	cleanUpProcess.add(customNamespacePolicy.Name, aclPolicyTestResourceType)

	// Create our default query options which can be used when testing a token
	// against the API. The caller should update the auth token as needed.
	defaultNSQueryMeta := api.QueryOptions{Namespace: "default"}

	// Attempt to create a token with a lower than acceptable TTL and ensure we
	// get an error.
	tokenTTLLow := api.ACLToken{
		Name:          "e2e-acl-" + uuid.Short(),
		Type:          "client",
		Policies:      []string{customNamespacePolicy.Name},
		ExpirationTTL: minTokenExpiryDur / 2,
	}
	aclTokenCreateResp, _, err := nomadClient.ACLTokens().Create(&tokenTTLLow, nil)
	require.ErrorContains(t, err, fmt.Sprintf(
		"expiration time cannot be less than %s in the future", minTokenExpiryDur))
	require.Nil(t, aclTokenCreateResp)

	// Attempt to create a token with a higher than acceptable TTL and ensure
	// we get an error.
	tokenTTLHigh := api.ACLToken{
		Name:          "e2e-acl-" + uuid.Short(),
		Type:          "client",
		Policies:      []string{customNamespacePolicy.Name},
		ExpirationTTL: 8766 * time.Hour,
	}
	aclTokenCreateResp, _, err = nomadClient.ACLTokens().Create(&tokenTTLHigh, nil)
	require.ErrorContains(t, err, fmt.Sprintf(
		"expiration time cannot be more than %s in the future", maxTokenExpiryDur))
	require.Nil(t, aclTokenCreateResp)

	// Create an ACL token that has a fairly standard TTL that will allow us to
	// make successful calls without it expiring.
	tokenNormalExpiry := api.ACLToken{
		Name:          "e2e-acl-" + uuid.Short(),
		Type:          "client",
		Policies:      []string{customNamespacePolicy.Name},
		ExpirationTTL: 10 * time.Minute,
	}
	tokenNormalExpiryCreateResp, _, err := nomadClient.ACLTokens().Create(&tokenNormalExpiry, nil)
	require.NoError(t, err)
	require.NotNil(t, tokenNormalExpiryCreateResp)
	require.Equal(t,
		*tokenNormalExpiryCreateResp.ExpirationTime,
		tokenNormalExpiryCreateResp.CreateTime.Add(tokenNormalExpiryCreateResp.ExpirationTTL))

	cleanUpProcess.add(tokenNormalExpiryCreateResp.AccessorID, aclTokenTestResourceType)

	// Add the token to our query options and ensure we can now list jobs with
	// the default namespace.
	defaultNSQueryMeta.AuthToken = tokenNormalExpiryCreateResp.SecretID

	jobListResp, _, err := nomadClient.Jobs().List(&defaultNSQueryMeta)
	require.NoError(t, err)
	require.Empty(t, jobListResp)

	// Create an ACL token with the lowest expiry TTL possible, so it will
	// expire almost immediately.
	tokenQuickExpiry := api.ACLToken{
		Name:          "e2e-acl-" + uuid.Short(),
		Type:          "client",
		Policies:      []string{customNamespacePolicy.Name},
		ExpirationTTL: minTokenExpiryDur,
	}
	tokenQuickExpiryCreateResp, _, err := nomadClient.ACLTokens().Create(&tokenQuickExpiry, nil)
	require.NoError(t, err)
	require.NotNil(t, tokenQuickExpiryCreateResp)
	require.Equal(t,
		*tokenQuickExpiryCreateResp.ExpirationTime,
		tokenQuickExpiryCreateResp.CreateTime.Add(tokenQuickExpiryCreateResp.ExpirationTTL))

	cleanUpProcess.add(tokenQuickExpiryCreateResp.AccessorID, aclTokenTestResourceType)

	// Block the test (sorry) until the token has expired.
	time.Sleep(tokenQuickExpiry.ExpirationTTL)

	// Update our query options and attempt to list the job using our expired
	// token.
	defaultNSQueryMeta.AuthToken = tokenQuickExpiryCreateResp.SecretID

	jobListResp, _, err = nomadClient.Jobs().List(&defaultNSQueryMeta)
	require.ErrorContains(t, err, "ACL token expired")
	require.Nil(t, jobListResp)

	// Force a run of the garbage collection process. This should remove all
	// expired tokens from state but leave unexpired tokens.
	require.NoError(t, nomadClient.System().GarbageCollect())

	tokenNormalExpiryReadResp, _, err := nomadClient.ACLTokens().Info(tokenNormalExpiryCreateResp.AccessorID, nil)
	require.NoError(t, err)
	require.NotNil(t, tokenNormalExpiryReadResp)
	require.Equal(t, tokenNormalExpiryCreateResp.SecretID, tokenNormalExpiryReadResp.SecretID)

	tokenQuickExpiryReadResp, _, err := nomadClient.ACLTokens().Info(tokenQuickExpiryCreateResp.AccessorID, nil)
	require.ErrorContains(t, err, "ACL token not found")
	require.Nil(t, tokenQuickExpiryReadResp)

	cleanUpProcess.remove(tokenQuickExpiryCreateResp.AccessorID, aclTokenTestResourceType)

	// Ensure we can manually delete unexpired tokens and that they are
	// immediately removed from state.
	_, err = nomadClient.ACLTokens().Delete(tokenNormalExpiryReadResp.AccessorID, nil)
	require.NoError(t, err)

	tokenNormalExpiryReadResp, _, err = nomadClient.ACLTokens().Info(tokenNormalExpiryCreateResp.AccessorID, nil)
	require.ErrorContains(t, err, "ACL token not found")

	cleanUpProcess.remove(tokenNormalExpiryCreateResp.AccessorID, aclTokenTestResourceType)
}

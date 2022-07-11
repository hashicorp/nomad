package nomad

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestSecureVariablesEndpoint_auth(t *testing.T) {

	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	const ns = "nondefault-namespace"

	alloc1 := mock.Alloc()
	alloc1.ClientStatus = structs.AllocClientStatusFailed
	alloc1.Job.Namespace = ns
	alloc1.Namespace = ns
	jobID := alloc1.JobID

	// create an alloc that will have no access to secure variables we create
	alloc2 := mock.Alloc()
	alloc2.Job.TaskGroups[0].Name = "other-no-permissions"
	alloc2.TaskGroup = "other-no-permissions"
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.Job.Namespace = ns
	alloc2.Namespace = ns

	store := srv.fsm.State()
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	claims1 := alloc1.ToTaskIdentityClaims("web")
	idToken, err := srv.encrypter.SignClaims(claims1)
	require.NoError(t, err)

	claims2 := alloc2.ToTaskIdentityClaims("web")
	noPermissionsToken, err := srv.encrypter.SignClaims(claims2)
	require.NoError(t, err)

	// corrupt the signature of the token
	idTokenParts := strings.Split(idToken, ".")
	require.Len(t, idTokenParts, 3)
	sig := []string(strings.Split(idTokenParts[2], ""))
	rand.Shuffle(len(sig), func(i, j int) {
		sig[i], sig[j] = sig[j], sig[i]
	})
	idTokenParts[2] = strings.Join(sig, "")
	invalidIDToken := strings.Join(idTokenParts, ".")

	policy := mock.ACLPolicy()
	policy.Name = fmt.Sprintf("_:%s/%s/%s", ns, jobID, alloc1.TaskGroup)
	policy.Rules = `namespace "nondefault-namespace" {
		secure_variables {
		    path "jobs/*" { capabilities = ["read"] }
		    path "other/path" { capabilities = ["read"] }
		}}`
	policy.SetHash()
	err = store.UpsertACLPolicies(structs.MsgTypeTestSetup, 1100, []*structs.ACLPolicy{policy})
	require.NoError(t, err)

	aclToken := mock.ACLToken()
	aclToken.Policies = []string{policy.Name}
	err = store.UpsertACLTokens(structs.MsgTypeTestSetup, 1150, []*structs.ACLToken{aclToken})
	require.NoError(t, err)

	t.Run("terminal alloc should be denied", func(t *testing.T) {
		err = srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
			structs.QueryOptions{AuthToken: idToken, Namespace: ns}, "n/a",
			fmt.Sprintf("jobs/%s/web/web", jobID))
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	// make alloc non-terminal
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1200, []*structs.Allocation{alloc1}))

	t.Run("wrong namespace should be denied", func(t *testing.T) {
		err = srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
			structs.QueryOptions{AuthToken: idToken, Namespace: structs.DefaultNamespace}, "n/a",
			fmt.Sprintf("jobs/%s/web/web", jobID))
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	testCases := []struct {
		name        string
		token       string
		cap         string
		path        string
		expectedErr error
	}{
		{
			name:        "valid claim for path with task secret",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with group secret",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with job secret",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with namespace secret",
			token:       idToken,
			cap:         "n/a",
			path:        "jobs",
			expectedErr: nil,
		},
		{
			name:        "valid claim for implied policy",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        "other/path",
			expectedErr: nil,
		},
		{
			name:        "valid claim for implied policy path denied",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        "other/not-allowed",
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "valid claim for implied policy capability denied",
			token:       idToken,
			cap:         acl.PolicyWrite,
			path:        "other/path",
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "valid claim with no permissions denied by path",
			token:       noPermissionsToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "valid claim with no permissions allowed by namespace",
			token:       noPermissionsToken,
			cap:         "n/a",
			path:        "jobs",
			expectedErr: nil,
		},
		{
			name:        "valid claim with no permissions denied by capability",
			token:       noPermissionsToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "extra trailing slash is denied",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/web/", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid prefix is denied",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "missing auth token is denied",
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid signature is denied",
			token:       invalidIDToken,
			cap:         "n/a",
			path:        fmt.Sprintf("jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "acl token read policy is allowed to list",
			token:       aclToken.SecretID,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "acl token read policy is not allowed to write",
			token:       aclToken.SecretID,
			cap:         acl.PolicyWrite,
			path:        fmt.Sprintf("jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
				structs.QueryOptions{AuthToken: tc.token, Namespace: ns}, tc.cap, tc.path)
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}

}

package nomad

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

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

	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusFailed
	alloc.Job.Namespace = ns
	jobID := alloc.JobID

	store := srv.fsm.State()
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	claim := alloc.ToTaskIdentityClaims("web")
	e := srv.encrypter

	idToken, err := e.SignClaims(claim)
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

	t.Run("terminal alloc should be denied", func(t *testing.T) {
		err = srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
			structs.QueryOptions{AuthToken: idToken, Namespace: ns}, "n/a",
			fmt.Sprintf("jobs/%s/web/web", jobID))
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	// make alloc non-terminal
	alloc.ClientStatus = structs.AllocClientStatusRunning
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1200, []*structs.Allocation{alloc}))

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

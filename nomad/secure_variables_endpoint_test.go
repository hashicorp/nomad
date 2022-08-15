package nomad

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
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

	alloc3 := mock.Alloc()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	alloc3.Job.Namespace = ns
	alloc3.Namespace = ns
	alloc3.Job.ParentID = jobID

	store := srv.fsm.State()
	must.NoError(t, store.UpsertNamespaces(1000, []*structs.Namespace{{Name: ns}}))
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1, alloc2, alloc3}))

	claims1 := alloc1.ToTaskIdentityClaims(nil, "web")
	idToken, err := srv.encrypter.SignClaims(claims1)
	must.NoError(t, err)

	claims2 := alloc2.ToTaskIdentityClaims(nil, "web")
	noPermissionsToken, err := srv.encrypter.SignClaims(claims2)
	must.NoError(t, err)

	claims3 := alloc3.ToTaskIdentityClaims(alloc3.Job, "web")
	idDispatchToken, err := srv.encrypter.SignClaims(claims3)
	must.NoError(t, err)

	// corrupt the signature of the token
	idTokenParts := strings.Split(idToken, ".")
	must.Len(t, 3, idTokenParts)
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
		    path "nomad/jobs/*" { capabilities = ["read"] }
		    path "other/path" { capabilities = ["read"] }
		}}`
	policy.SetHash()
	err = store.UpsertACLPolicies(structs.MsgTypeTestSetup, 1100, []*structs.ACLPolicy{policy})
	must.NoError(t, err)

	aclToken := mock.ACLToken()
	aclToken.Policies = []string{policy.Name}
	err = store.UpsertACLTokens(structs.MsgTypeTestSetup, 1150, []*structs.ACLToken{aclToken})
	must.NoError(t, err)

	t.Run("terminal alloc should be denied", func(t *testing.T) {
		err = srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
			structs.QueryOptions{AuthToken: idToken, Namespace: ns}, "n/a",
			fmt.Sprintf("nomad/jobs/%s/web/web", jobID))
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})

	// make alloc non-terminal
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1200, []*structs.Allocation{alloc1}))

	t.Run("wrong namespace should be denied", func(t *testing.T) {
		err = srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
			structs.QueryOptions{AuthToken: idToken, Namespace: structs.DefaultNamespace}, "n/a",
			fmt.Sprintf("nomad/jobs/%s/web/web", jobID))
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
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
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with group secret",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with job secret",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with dispatch job secret",
			token:       idDispatchToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s", jobID),
			expectedErr: nil,
		},
		{
			name:        "valid claim for path with namespace secret",
			token:       idToken,
			cap:         "n/a",
			path:        "nomad/jobs",
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
			path:        fmt.Sprintf("nomad/jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "valid claim with no permissions allowed by namespace",
			token:       noPermissionsToken,
			cap:         "n/a",
			path:        "nomad/jobs",
			expectedErr: nil,
		},
		{
			name:        "valid claim with no permissions denied by capability",
			token:       noPermissionsToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "extra trailing slash is denied",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s/web/", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid prefix is denied",
			token:       idToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "missing auth token is denied",
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid signature is denied",
			token:       invalidIDToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid claim for dispatched ID",
			token:       idDispatchToken,
			cap:         "n/a",
			path:        fmt.Sprintf("nomad/jobs/%s", alloc3.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "acl token read policy is allowed to list",
			token:       aclToken.SecretID,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "acl token read policy is not allowed to write",
			token:       aclToken.SecretID,
			cap:         acl.PolicyWrite,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := srv.staticEndpoints.SecureVariables.handleMixedAuthEndpoint(
				structs.QueryOptions{AuthToken: tc.token, Namespace: ns}, tc.cap, tc.path)
			if tc.expectedErr == nil {
				must.NoError(t, err)
			} else {
				must.EqError(t, err, tc.expectedErr.Error())
			}
		})
	}

}

func TestSecureVariablesEndpoint_Apply_ACL(t *testing.T) {
	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()

	pol := mock.NamespacePolicyWithSecureVariables(
		structs.DefaultNamespace, "", []string{"list-jobs"},
		map[string][]string{
			"dropbox/*": {"write"},
		})
	writeToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", pol)

	sv1 := mock.SecureVariable()
	sv1.ModifyIndex = 0
	var svHold *structs.SecureVariableDecrypted

	opMap := map[string]structs.SVOp{
		"set":        structs.SVOpSet,
		"cas":        structs.SVOpCAS,
		"delete":     structs.SVOpDelete,
		"delete-cas": structs.SVOpDeleteCAS,
	}

	for name, op := range opMap {
		t.Run(name+"/no token", func(t *testing.T) {
			sv1 := sv1
			applyReq := structs.SecureVariablesApplyRequest{
				Op:           op,
				Var:          sv1,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			applyResp := new(structs.SecureVariablesApplyResponse)
			err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, applyResp)
			must.EqError(t, err, structs.ErrPermissionDenied.Error())
		})
	}

	t.Run("cas/management token/new", func(t *testing.T) {
		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultOk, applyResp.Result)
		must.Equals(t, sv1.Items, applyResp.Output.Items)

		svHold = applyResp.Output
	})

	t.Run("cas with current", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := svHold
		sv.Items["new"] = "newVal"

		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)
		applyReq.AuthToken = rootToken.SecretID

		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultOk, applyResp.Result)
		must.Equals(t, sv.Items, applyResp.Output.Items)

		svHold = applyResp.Output
	})

	t.Run("cas with stale", func(t *testing.T) {
		must.NotNil(t, sv1) // TODO: query these directly
		must.NotNil(t, svHold)

		sv1 := sv1
		svHold := svHold

		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)
		applyReq.AuthToken = rootToken.SecretID

		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultConflict, applyResp.Result)
		must.Equals(t, svHold.SecureVariableMetadata, applyResp.Conflict.SecureVariableMetadata)
		must.Equals(t, svHold.Items, applyResp.Conflict.Items)
	})

	sv3 := mock.SecureVariable()
	sv3.Path = "dropbox/a"
	sv3.ModifyIndex = 0

	t.Run("cas/write-only/read own new", func(t *testing.T) {
		sv3 := sv3
		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)

		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultOk, applyResp.Result)
		must.Equals(t, sv3.Items, applyResp.Output.Items)
		svHold = applyResp.Output
	})

	t.Run("cas/write only/conflict redacted", func(t *testing.T) {
		must.NotNil(t, sv3)
		must.NotNil(t, svHold)
		sv3 := sv3
		svHold := svHold

		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultRedacted, applyResp.Result)
		must.Eq(t, svHold.SecureVariableMetadata, applyResp.Conflict.SecureVariableMetadata)
		must.Nil(t, applyResp.Conflict.Items)
	})

	t.Run("cas/write only/read own upsert", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := svHold
		sv.Items["upsert"] = "read"

		applyReq := structs.SecureVariablesApplyRequest{
			Op:  structs.SVOpCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SecureVariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.SVOpResultOk, applyResp.Result)
		must.Equals(t, sv.Items, applyResp.Output.Items)
	})
}

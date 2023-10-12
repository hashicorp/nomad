// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/auth"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestVariablesEndpoint_GetVariable_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// First create an unrelated variable.
	delay := 100 * time.Millisecond
	time.AfterFunc(delay, func() {
		writeVarGet(t, s1, 100, "default", "aaa")
	})

	// Upsert the variable we are watching later
	delay = 200 * time.Millisecond
	time.AfterFunc(delay, func() {
		writeVarGet(t, s1, 200, "default", "bbb")
	})

	// Lookup the variable
	req := &structs.VariablesReadRequest{
		Path: "bbb",
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			MaxQueryTime:  500 * time.Millisecond,
		},
	}

	var resp structs.VariablesReadResponse

	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Variables.Read", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < delay {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	if elapsed > req.MaxQueryTime {
		t.Fatalf("blocking query timed out %#v", resp)
	}

	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}

	if resp.Data == nil || resp.Data.Path != "bbb" {
		t.Fatalf("bad: %#v", resp.Data)
	}

	// Variable update triggers watches
	delay = 100 * time.Millisecond

	time.AfterFunc(delay, func() {
		writeVarGet(t, s1, 300, "default", "bbb")
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.VariablesReadResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Variables.Read", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	elapsed = time.Since(start)

	if elapsed < delay {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}

	if elapsed > req.MaxQueryTime {
		t.Fatal("blocking query timed out")
	}

	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}

	if resp2.Data == nil || resp2.Data.Path != "bbb" {
		t.Fatalf("bad: %#v", resp2.Data)
	}

	// Variable delete triggers watches
	delay = 100 * time.Millisecond
	time.AfterFunc(delay, func() {
		sv := mock.VariableEncrypted()
		sv.Path = "bbb"
		if resp := state.VarDelete(400, &structs.VarApplyStateRequest{Op: structs.VarOpDelete, Var: sv}); !resp.IsOk() {
			fmt.Println("\n *** resp", resp.Result, resp.Conflict.VariableMetadata)
			t.Fatalf("err: %v", resp.Error)
		}
	})
	fmt.Println("\n\n start", elapsed)
	req.QueryOptions.MinQueryIndex = 350
	var resp3 structs.VariablesReadResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Variables.Read", req, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	elapsed = time.Since(start)
	fmt.Println("\n\n problematic", elapsed)
	if elapsed < delay {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	if elapsed > req.MaxQueryTime {
		t.Fatal("blocking query timed out")
	}

	if resp3.Index != 400 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 400)
	}
	if resp3.Data != nil {
		t.Fatalf("bad: %#v", resp3.Data)
	}
}

func writeVarGet(t *testing.T, s *Server, idx uint64, ns, path string) {
	store := s.fsm.State()
	sv := mock.Variable()
	sv.Namespace = ns
	sv.Path = path

	bPlain, err := json.Marshal(sv.Items)
	must.NoError(t, err)
	bEnc, kID, err := s.encrypter.Encrypt(bPlain)
	must.NoError(t, err)
	sve := &structs.VariableEncrypted{
		VariableMetadata: sv.VariableMetadata,
		VariableData: structs.VariableData{
			Data:  bEnc,
			KeyID: kID,
		},
	}
	resp := store.VarSet(idx, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: sve,
	})
	must.NoError(t, resp.Error)
}

func TestVariablesEndpoint_Apply_ACL(t *testing.T) {
	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()

	writePol := mock.NamespacePolicyWithVariables(
		structs.DefaultNamespace, "", []string{"list-jobs"},
		map[string][]string{
			"dropbox/*": {"write"},
		})
	writeToken := mock.CreatePolicyAndToken(t, state, 1003, "test-write-invalid", writePol)

	readPol := mock.NamespacePolicyWithVariables(
		structs.DefaultNamespace, "", []string{"list-jobs"},
		map[string][]string{
			"dropbox/*": {"read"},
		})

	readToken := mock.CreatePolicyAndToken(t, state, 1005, "test-read-invalid", readPol)

	sv1 := mock.Variable()
	sv1.ModifyIndex = 0
	var svHold *structs.VariableDecrypted

	opMap := map[string]structs.VarOp{
		"set":          structs.VarOpSet,
		"cas":          structs.VarOpCAS,
		"delete":       structs.VarOpDelete,
		"delete-cas":   structs.VarOpDeleteCAS,
		"lock-acquire": structs.VarOpLockAcquire,
		"lock-release": structs.VarOpLockRelease,
	}

	for name, op := range opMap {
		t.Run(name+"/no token", func(t *testing.T) {
			sv := *sv1
			applyReq := structs.VariablesApplyRequest{
				Op:           op,
				Var:          &sv,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}

			if op == "lock-release" {
				sv.Items = nil
			}

			applyResp := new(structs.VariablesApplyResponse)
			err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)
			must.EqError(t, err, structs.ErrPermissionDenied.Error())
		})

	}

	t.Run("cas/management token/new", func(t *testing.T) {
		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv1.Items, applyResp.Output.Items)

		svHold = applyResp.Output
	})

	t.Run("cas with current", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := svHold
		sv.Items["new"] = "newVal"

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)
		applyReq.AuthToken = rootToken.SecretID

		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv.Items, applyResp.Output.Items)

		svHold = applyResp.Output
	})

	t.Run("cas with stale", func(t *testing.T) {
		must.NotNil(t, sv1) // TODO: query these directly
		must.NotNil(t, svHold)

		sv1 := sv1
		svHold := svHold

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)
		applyReq.AuthToken = rootToken.SecretID

		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
		must.Eq(t, svHold.VariableMetadata, applyResp.Conflict.VariableMetadata)
		must.Eq(t, svHold.Items, applyResp.Conflict.Items)
	})

	sv3 := mock.Variable()
	sv3.Path = "dropbox/a"
	sv3.ModifyIndex = 0

	t.Run("cas/write-only/read own new", func(t *testing.T) {
		sv3 := sv3
		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)

		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv3.Items, applyResp.Output.Items)
		svHold = applyResp.Output
	})

	t.Run("cas/write only/conflict redacted", func(t *testing.T) {
		must.NotNil(t, sv3)
		must.NotNil(t, svHold)
		sv3 := sv3
		svHold := svHold

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultRedacted, applyResp.Result)
		must.Eq(t, svHold.VariableMetadata, applyResp.Conflict.VariableMetadata)
		must.Nil(t, applyResp.Conflict.Items)
	})

	t.Run("cas/write only/read own upsert", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := svHold
		sv.Items["upsert"] = "read"

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv.Items, applyResp.Output.Items)
	})

	t.Run("lock-acquire/management token/new", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := *svHold

		sv.Items["upsert"] = "read"
		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpLockAcquire,
			Var: &sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.NonZero(t, len(applyResp.Output.VariableMetadata.Lock.ID))

		svHold = applyResp.Output
	})

	t.Run("lock-release/management token/new", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := svHold

		applyReq := structs.VariablesApplyRequest{
			Op: structs.VarOpLockRelease,
			Var: &structs.VariableDecrypted{
				VariableMetadata: structs.VariableMetadata{
					Path: sv.Path,
					Lock: &structs.VariableLock{
						ID: sv.LockID(),
					},
				},
			},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Nil(t, applyResp.Output.VariableMetadata.Lock)
	})

	t.Run("lock-acquire/read token/new", func(t *testing.T) {
		must.NotNil(t, svHold)
		sv := *sv3

		sv.Items["upsert"] = "read"
		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpLockAcquire,
			Var: &sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: readToken.SecretID,
			},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, &applyResp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})
}

func TestVariablesEndpoint_auth(t *testing.T) {

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

	// create an alloc that will have no access to variables we create
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
	parentID := uuid.Short()
	alloc3.Job.ParentID = parentID

	alloc4 := mock.Alloc()
	alloc4.ClientStatus = structs.AllocClientStatusRunning
	alloc4.Job.Namespace = ns
	alloc4.Namespace = ns

	store := srv.fsm.State()
	must.NoError(t, store.UpsertNamespaces(1000, []*structs.Namespace{{Name: ns}}))
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}))

	wiHandle := &structs.WIHandle{
		WorkloadIdentifier: "web",
		WorkloadType:       structs.WorkloadTypeTask,
	}

	claims1 := structs.NewIdentityClaims(alloc1.Job, alloc1, wiHandle, alloc1.LookupTask("web").Identity, time.Now())
	idToken, _, err := srv.encrypter.SignClaims(claims1)
	must.NoError(t, err)

	claims2 := structs.NewIdentityClaims(alloc2.Job, alloc2, wiHandle, alloc2.LookupTask("web").Identity, time.Now())
	noPermissionsToken, _, err := srv.encrypter.SignClaims(claims2)
	must.NoError(t, err)

	claims3 := structs.NewIdentityClaims(alloc3.Job, alloc3, wiHandle, alloc3.LookupTask("web").Identity, time.Now())
	idDispatchToken, _, err := srv.encrypter.SignClaims(claims3)
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

	claims4 := structs.NewIdentityClaims(alloc4.Job, alloc4, wiHandle, alloc4.LookupTask("web").Identity, time.Now())
	wiOnlyToken, _, err := srv.encrypter.SignClaims(claims4)
	must.NoError(t, err)

	policy := mock.ACLPolicy()
	policy.Rules = fmt.Sprintf(`namespace "nondefault-namespace" {
		variables {
		    path "nomad/jobs/*" { capabilities = ["list"] }
		    path "nomad/jobs/%s/web" { capabilities = ["deny"] }
		    path "nomad/jobs/%s" { capabilities = ["write"] }
		    path "other/path" { capabilities = ["read"] }
		}}`, jobID, jobID)
	policy.JobACL = &structs.JobACL{
		Namespace: ns,
		JobID:     jobID,
		Group:     alloc1.TaskGroup,
	}
	policy.SetHash()
	err = store.UpsertACLPolicies(structs.MsgTypeTestSetup, 1100, []*structs.ACLPolicy{policy})
	must.NoError(t, err)

	aclToken := mock.ACLToken()
	aclToken.Policies = []string{policy.Name}
	err = store.UpsertACLTokens(structs.MsgTypeTestSetup, 1150, []*structs.ACLToken{aclToken})
	must.NoError(t, err)

	testFn := func(args *structs.QueryOptions, cap, path string) error {
		err := srv.Authenticate(nil, args)
		if err != nil {
			return structs.ErrPermissionDenied
		}
		aclObj, err := srv.ResolveACL(args)
		if err != nil {
			return err
		}
		if !aclObj.AllowVariableOperation(args.Namespace, path, cap,
			auth.IdentityToACLClaim(args.GetIdentity(), srv.State())) {
			return structs.ErrPermissionDenied
		}

		return nil
	}

	t.Run("terminal alloc should be denied", func(t *testing.T) {
		err := testFn(
			&structs.QueryOptions{AuthToken: idToken, Namespace: ns}, acl.PolicyList,
			fmt.Sprintf("nomad/jobs/%s/web/web", jobID))
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})

	// make alloc non-terminal
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 1200, []*structs.Allocation{alloc1}))

	t.Run("wrong namespace should be denied", func(t *testing.T) {
		err := testFn(&structs.QueryOptions{
			AuthToken: idToken, Namespace: structs.DefaultNamespace}, acl.PolicyList,
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
			name:        "WI with policy no override can read task secret",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "WI with policy no override can list task secret",
			token:       idToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: nil,
		},
		{
			name:        "WI with policy override denies list group secret",
			token:       idToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI with policy override can write job secret",
			token:       idToken,
			cap:         acl.PolicyWrite,
			path:        fmt.Sprintf("nomad/jobs/%s", jobID),
			expectedErr: nil,
		},
		{
			name:        "WI with policy override for write-only job secret",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI with policy no override can list namespace secret",
			token:       idToken,
			cap:         acl.PolicyList,
			path:        "nomad/jobs",
			expectedErr: nil,
		},

		{
			name:        "WI with policy can read other path",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        "other/path",
			expectedErr: nil,
		},
		{
			name:        "WI with policy cannot read other path not explicitly allowed",
			token:       idToken,
			cap:         acl.PolicyRead,
			path:        "other/not-allowed",
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI with policy has no write cap for other path",
			token:       idToken,
			cap:         acl.PolicyWrite,
			path:        "other/path",
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI with policy can read cross-job path",
			token:       idToken,
			cap:         acl.PolicyList,
			path:        "nomad/jobs/some-other",
			expectedErr: nil,
		},

		{
			name:        "WI for dispatch job can read parent secret",
			token:       idDispatchToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s", parentID),
			expectedErr: nil,
		},

		{
			name:        "valid claim with no permissions denied by path",
			token:       noPermissionsToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/w", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "valid claim with no permissions allowed by namespace",
			token:       noPermissionsToken,
			cap:         acl.PolicyList,
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
			name:        "missing auth token is denied",
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid signature is denied",
			token:       invalidIDToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", jobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "invalid claim for dispatched ID",
			token:       idDispatchToken,
			cap:         acl.PolicyList,
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

		{
			name:        "WI token can read own task",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", alloc4.JobID),
			expectedErr: nil,
		},
		{
			name:        "WI token can list own task",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/web", alloc4.JobID),
			expectedErr: nil,
		},
		{
			name:        "WI token can read own group",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/web", alloc4.JobID),
			expectedErr: nil,
		},
		{
			name:        "WI token can list own group",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web", alloc4.JobID),
			expectedErr: nil,
		},

		{
			name:        "WI token cannot read another task in group",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/web/other", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot list another task in group",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/other", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot read another task in group",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/web/other", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot list a task in another group",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/other/web", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot read a task in another group",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        fmt.Sprintf("nomad/jobs/%s/other/web", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot read a group in another job",
			token:       wiOnlyToken,
			cap:         acl.PolicyRead,
			path:        "nomad/jobs/other/web/web",
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token cannot list a group in another job",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        "nomad/jobs/other/web/web",
			expectedErr: structs.ErrPermissionDenied,
		},

		{
			name:        "WI token extra trailing slash is denied",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/web/", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
		{
			name:        "WI token invalid prefix is denied",
			token:       wiOnlyToken,
			cap:         acl.PolicyList,
			path:        fmt.Sprintf("nomad/jobs/%s/w", alloc4.JobID),
			expectedErr: structs.ErrPermissionDenied,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := testFn(
				&structs.QueryOptions{AuthToken: tc.token, Namespace: ns},
				tc.cap, tc.path)
			if tc.expectedErr == nil {
				must.NoError(t, err)
			} else {
				must.EqError(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestVariablesEndpoint_ListFiltering(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	ns := "nondefault-namespace"
	idx := uint64(1000)

	alloc := mock.Alloc()
	alloc.Job.ID = "job1"
	alloc.JobID = "job1"
	alloc.TaskGroup = "group"
	alloc.Job.TaskGroups[0].Name = "group"
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.Job.Namespace = ns
	alloc.Namespace = ns

	store := srv.fsm.State()
	must.NoError(t, store.UpsertNamespaces(idx, []*structs.Namespace{{Name: ns}}))
	idx++
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, idx, []*structs.Allocation{alloc}))

	wiHandle := &structs.WIHandle{
		WorkloadIdentifier: "web",
		WorkloadType:       structs.WorkloadTypeTask,
	}

	claims := structs.NewIdentityClaims(alloc.Job, alloc, wiHandle, alloc.LookupTask("web").Identity, time.Now())
	token, _, err := srv.encrypter.SignClaims(claims)
	must.NoError(t, err)

	writeVar := func(ns, path string) {
		idx++
		sv := mock.VariableEncrypted()
		sv.Namespace = ns
		sv.Path = path
		resp := store.VarSet(idx, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		must.NoError(t, resp.Error)
	}

	writeVar(ns, "nomad/jobs/job1/group/web")
	writeVar(ns, "nomad/jobs/job1/group")
	writeVar(ns, "nomad/jobs/job1")

	writeVar(ns, "nomad/jobs/job1/group/other")
	writeVar(ns, "nomad/jobs/job1/other/web")
	writeVar(ns, "nomad/jobs/job2/group/web")

	req := &structs.VariablesListRequest{
		QueryOptions: structs.QueryOptions{
			Namespace: ns,
			Prefix:    "nomad",
			AuthToken: token,
			Region:    "global",
		},
	}
	var resp structs.VariablesListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &resp))
	found := []string{}
	for _, variable := range resp.Data {
		found = append(found, variable.Path)
	}
	expect := []string{
		"nomad/jobs/job1",
		"nomad/jobs/job1/group",
		"nomad/jobs/job1/group/web",
	}
	must.Eq(t, expect, found)

	// Associate a policy with the identity's job to deny partial access.
	policy := &structs.ACLPolicy{
		Name: "policy-for-identity",
		Rules: mock.NamespacePolicyWithVariables(ns, "read", []string{},
			map[string][]string{"nomad/jobs/job1/group": []string{"deny"}}),
		JobACL: &structs.JobACL{
			Namespace: ns,
			JobID:     "job1",
		},
	}
	policy.SetHash()
	must.NoError(t, store.UpsertACLPolicies(structs.MsgTypeTestSetup, 16,
		[]*structs.ACLPolicy{policy}))

	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &resp))
	found = []string{}
	for _, variable := range resp.Data {
		found = append(found, variable.Path)
	}
	expect = []string{
		"nomad/jobs/job1",
		"nomad/jobs/job1/group/web",
	}
	must.Eq(t, expect, found)
}

func TestVariablesEndpoint_ComplexACLPolicies(t *testing.T) {

	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	idx := uint64(1000)

	policyRules := `
namespace "dev" {
  variables {
    path "*" { capabilities = ["list", "read"] }
    path "system/*" { capabilities = ["deny"] }
    path "config/system/*" { capabilities = ["deny"] }
  }
}

namespace "prod" {
  variables {
    path  "*" {
    capabilities = ["list"]
    }
  }
}

namespace "*" {}
`

	store := srv.fsm.State()

	must.NoError(t, store.UpsertNamespaces(1000, []*structs.Namespace{
		{Name: "dev"}, {Name: "prod"}, {Name: "other"}}))

	idx++
	token := mock.CreatePolicyAndToken(t, store, idx, "developer", policyRules)

	writeVar := func(ns, path string) {
		idx++
		sv := mock.VariableEncrypted()
		sv.Namespace = ns
		sv.Path = path
		resp := store.VarSet(idx, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		must.NoError(t, resp.Error)
	}

	writeVar("dev", "system/never-list")
	writeVar("dev", "config/system/never-list")
	writeVar("dev", "config/can-read")
	writeVar("dev", "project/can-read")

	writeVar("prod", "system/can-list")
	writeVar("prod", "config/system/can-list")
	writeVar("prod", "config/can-list")
	writeVar("prod", "project/can-list")

	writeVar("other", "system/never-list")
	writeVar("other", "config/system/never-list")
	writeVar("other", "config/never-list")
	writeVar("other", "project/never-list")

	testListPrefix := func(ns, prefix string, expectedCount int, expectErr error) {
		t.Run(fmt.Sprintf("ns=%s-prefix=%s", ns, prefix), func(t *testing.T) {
			req := &structs.VariablesListRequest{
				QueryOptions: structs.QueryOptions{
					Namespace: ns,
					Prefix:    prefix,
					AuthToken: token.SecretID,
					Region:    "global",
				},
			}
			var resp structs.VariablesListResponse

			if expectErr != nil {
				must.EqError(t,
					msgpackrpc.CallWithCodec(codec, "Variables.List", req, &resp),
					expectErr.Error())
				return
			}
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &resp))

			found := "found:\n"
			for _, sv := range resp.Data {
				found += fmt.Sprintf(" ns=%s path=%s\n", sv.Namespace, sv.Path)
			}
			must.Len(t, expectedCount, resp.Data, must.Sprintf("%s", found))
		})
	}

	testListPrefix("dev", "system", 0, nil)
	testListPrefix("dev", "config/system", 0, nil)
	testListPrefix("dev", "config", 1, nil)
	testListPrefix("dev", "project", 1, nil)
	testListPrefix("dev", "", 2, nil)

	testListPrefix("prod", "system", 1, nil)
	testListPrefix("prod", "config/system", 1, nil)
	testListPrefix("prod", "config", 2, nil)
	testListPrefix("prod", "project", 1, nil)
	testListPrefix("prod", "", 4, nil)

	// list gives empty but no error!
	testListPrefix("other", "system", 0, nil)
	testListPrefix("other", "config/system", 0, nil)
	testListPrefix("other", "config", 0, nil)
	testListPrefix("other", "project", 0, nil)
	testListPrefix("other", "", 0, nil)

	testListPrefix("*", "system", 1, nil)
	testListPrefix("*", "config/system", 1, nil)
	testListPrefix("*", "config", 3, nil)
	testListPrefix("*", "project", 2, nil)
	testListPrefix("*", "", 6, nil)
}

func TestVariablesEndpoint_Apply_LockAcquireUpsertAndRelease(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	mockVar := mock.Variable()
	mockVar.ModifyIndex = 0

	latest := mockVar.Copy()

	t.Run("successfully acquire lock on new variable", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockAcquire,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.NonZero(t, len(applyResp.Output.VariableMetadata.Lock.ID))
		must.Eq(t, sv.Items, applyResp.Output.Items)

		must.NotNil(t, srv.lockTTLTimer.Get(applyResp.Output.VariableMetadata.Lock.ID))
		must.Eq(t, runningTimers+1, srv.lockTTLTimer.TimerNum())

		latest = applyResp.Output.Copy()
	})

	t.Run("acquire lock on locked variable without lockID", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockAcquire,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, runningTimers, srv.lockTTLTimer.TimerNum())
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
	})

	t.Run("successfully re acquire lock on locked variable with the lockID", func(t *testing.T) {
		sv := latest.Copy()

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockAcquire,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv.Items, applyResp.Output.Items)

		must.Eq(t, runningTimers, srv.lockTTLTimer.TimerNum())

		latest = applyResp.Output.Copy()
	})

	t.Run("upsert locked variable without the lockID", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.Items = structs.VariableItems{
			"item1": "very important info",
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpSet,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
	})

	t.Run("successfully upsert locked variable with the lockID", func(t *testing.T) {
		sv := latest.Copy()
		sv.Items = structs.VariableItems{
			"item1": "very important info",
		}

		sv.Lock.TTL = 0
		sv.Lock.LockDelay = 0

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpSet,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv.Items, applyResp.Output.Items)

		latest = applyResp.Output.Copy()
	})

	t.Run("upsert locked variable with correct CAS without the lockID", func(t *testing.T) {
		sv := latest.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{}
		sv.Items = structs.VariableItems{
			"item1": "very important info",
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpCAS,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
	})

	t.Run("upsert locked variable with wrong CAS without the lockID", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{}
		sv.Items = structs.VariableItems{
			"item1": "very important info",
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpCAS,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
	})

	t.Run("upsert locked variable wrong CAS and the lockID", func(t *testing.T) {
		sv := latest.Copy()

		sv.Items = structs.VariableItems{
			"item1": "very important info",
			"item2": "not so important info",
		}

		sv.Lock.LockDelay = 0
		sv.Lock.TTL = 0
		sv.ModifyIndex = sv.ModifyIndex + 30

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpCAS,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
	})

	t.Run("successfully upsert locked variable with CAS and the lockID", func(t *testing.T) {
		sv := latest.Copy()

		sv.Items = structs.VariableItems{
			"item1": "very important info",
			"item2": "not so important info",
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpCAS,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, sv.Items, applyResp.Output.Items)

		latest = applyResp.Output.Copy()
	})

	t.Run("release locked variable without the lock", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{
			ID: "wrongID",
		}

		sv.Items = nil
		sv.Lock = nil

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockRelease,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.Error(t, err)
		must.Eq(t, runningTimers, srv.lockTTLTimer.TimerNum())

	})
	t.Run("release locked variable without the lockID", func(t *testing.T) {
		sv := mockVar.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{
			ID: "wrongID",
		}
		sv.Items = nil

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockRelease,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultConflict, applyResp.Result)
		must.Eq(t, runningTimers, srv.lockTTLTimer.TimerNum())

	})

	t.Run("successfully release locked variable with the lockID", func(t *testing.T) {
		sv := latest.Copy()

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockRelease,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		sv.Items = nil

		runningTimers := srv.lockTTLTimer.TimerNum()
		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, runningTimers-1, srv.lockTTLTimer.TimerNum())

		must.Nil(t, applyResp.Output.Lock)
		must.Zero(t, len(applyResp.Output.Items))

		latest = applyResp.Output.Copy()
	})

	t.Run("successfully acquire lock on variable with new data", func(t *testing.T) {
		sv := latest.Copy()

		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		sv.Items = structs.VariableItems{
			"item1": "very important info",
			"item2": "not so important info",
			"item3": "the password",
		}

		applyReq := structs.VariablesApplyRequest{
			Op:           structs.VarOpLockAcquire,
			Var:          &sv,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()
		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.Eq(t, runningTimers+1, srv.lockTTLTimer.TimerNum())
		must.Eq(t, sv.Items, applyResp.Output.Items)

		latest = applyResp.Output.Copy()
	})
}

func TestVariablesEndpoint_List_Lock_ACL(t *testing.T) {
	ci.Parallel(t)

	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()

	listPol := mock.NamespacePolicyWithVariables(
		structs.DefaultNamespace, "", []string{"list-jobs"},
		map[string][]string{
			"dropbox/*": {"list", "read", "write"},
		})
	listToken := mock.CreatePolicyAndToken(t, state, 1003, "test-write-invalid", listPol)

	sv1 := mock.VariableEncrypted()
	sv1.Path = "dropbox/a"

	sv1.VariableMetadata.Lock = &structs.VariableLock{
		ID:        "theLockID",
		TTL:       24 * time.Hour,
		LockDelay: 15 * time.Second,
	}

	// Creating and locking the variable directly on the state store allows us to
	// set up the lock ID and bypass the timers.
	ssResp := state.VarLockAcquire(100, &structs.VarApplyStateRequest{
		Op:  structs.VarOpLockAcquire,
		Var: sv1,
	})

	must.NoError(t, ssResp.Error)

	t.Run("successfully read the lock information with a management call", func(t *testing.T) {
		req := &structs.VariablesListRequest{
			QueryOptions: structs.QueryOptions{
				Namespace: structs.DefaultNamespace,
				Prefix:    "dropbox",
				AuthToken: rootToken.SecretID,
				Region:    "global",
			},
		}

		listResp := new(structs.VariablesListResponse)
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &listResp))
		must.NotNil(t, listResp.Data[0].Lock)
		must.Eq(t, "theLockID", listResp.Data[0].Lock.ID)
	})

	t.Run("try to read the lock information without a token", func(t *testing.T) {
		req := &structs.VariablesListRequest{
			QueryOptions: structs.QueryOptions{
				Namespace: structs.DefaultNamespace,
				Prefix:    "dropbox",
				Region:    "global",
			},
		}

		listResp := new(structs.VariablesListResponse)
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &listResp))
		must.Zero(t, len(listResp.Data))
	})

	t.Run("try to read the lock information without a list token", func(t *testing.T) {
		req := &structs.VariablesListRequest{
			QueryOptions: structs.QueryOptions{
				Namespace: structs.DefaultNamespace,
				Prefix:    "dropbox",
				AuthToken: listToken.SecretID,
				Region:    "global",
			},
		}

		listResp := new(structs.VariablesListResponse)
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List", req, &listResp))
		must.NonZero(t, len(listResp.Data))
		must.Nil(t, listResp.Data[0].Lock)
	})
}

func TestVariablesEndpoint_Read_Lock_ACL(t *testing.T) {
	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()

	listPol := mock.NamespacePolicyWithVariables(
		structs.DefaultNamespace, "", []string{"list-jobs"},
		map[string][]string{
			"dropbox/*": {"list", "read"},
		})
	listToken := mock.CreatePolicyAndToken(t, state, 1003, "test-write-invalid", listPol)

	sv := mock.Variable()
	sv.Path = "dropbox/a"

	latest := sv.Copy()
	t.Run("successfully acquire lock on new variable", func(t *testing.T) {
		sv := sv.Copy()
		sv.VariableMetadata.Lock = &structs.VariableLock{
			TTL:       24 * time.Hour,
			LockDelay: 15 * time.Second,
		}

		applyReq := structs.VariablesApplyRequest{
			Op:  structs.VarOpLockAcquire,
			Var: &sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}

		runningTimers := srv.lockTTLTimer.TimerNum()

		applyResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesApplyRPCMethod, &applyReq, applyResp)

		must.NoError(t, err)
		must.Eq(t, structs.VarOpResultOk, applyResp.Result)
		must.NotNil(t, applyResp.Output.VariableMetadata.Lock)
		must.Eq(t, sv.Items, applyResp.Output.Items)

		must.NotNil(t, srv.lockTTLTimer.Get(applyResp.Output.VariableMetadata.Lock.ID))
		must.Eq(t, runningTimers+1, srv.lockTTLTimer.TimerNum())
		latest = *applyResp.Output

	})

	t.Run("successfully read the lock information with a management call", func(t *testing.T) {
		req := &structs.VariablesReadRequest{
			Path: sv.Path,
			QueryOptions: structs.QueryOptions{
				Namespace: structs.DefaultNamespace,
				AuthToken: rootToken.SecretID,
				Region:    "global",
			},
		}

		readResp := new(structs.VariablesReadResponse)
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.Read", req, &readResp))
		must.NotNil(t, readResp.Data.VariableMetadata.Lock)
		must.Eq(t, latest.LockID(), readResp.Data.LockID())
	})

	t.Run("try to read the lock information with a list token", func(t *testing.T) {
		req := &structs.VariablesReadRequest{
			Path: sv.Path,
			QueryOptions: structs.QueryOptions{
				Namespace: structs.DefaultNamespace,
				AuthToken: listToken.SecretID,
				Region:    "global",
			},
		}

		readResp := new(structs.VariablesReadResponse)
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.Read", req, &readResp))
		must.Nil(t, readResp.Data.VariableMetadata.Lock)
	})
}

func TestVariablesEndpoint_RenewLock(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()

	unlockedVar := mock.VariableEncrypted()

	vsResp := state.VarSet(102, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: unlockedVar,
	})
	must.NoError(t, vsResp.Error)

	lockedVar := mock.VariableEncrypted()
	lockedVar.VariableMetadata.Lock = &structs.VariableLock{
		ID:        "theLockID",
		TTL:       24 * time.Hour,
		LockDelay: 15 * time.Second,
	}

	// Creating and locking the variable directly on the state store allows us to
	// set up the lock ID and bypass the timers.
	vlResp := state.VarLockAcquire(104, &structs.VarApplyStateRequest{
		Op:  structs.VarOpLockAcquire,
		Var: lockedVar,
	})

	must.NoError(t, vlResp.Error)

	t.Run("error renewing lock", func(t *testing.T) {

		testCases := []struct {
			name     string
			varPath  string
			lockID   string
			expError error
		}{
			{
				name:     "renewing lock on a missing variable",
				varPath:  "missing/variable/path",
				lockID:   "missingVariableID",
				expError: errVarNotFound,
			},
			{
				name:     "renewing lock on unlocked variable",
				varPath:  unlockedVar.Path,
				lockID:   "randomLockID",
				expError: errLockNotFound,
			},
			{
				name:     "renewing lock with the wrong lockID",
				varPath:  lockedVar.Path,
				lockID:   "wrongLockID",
				expError: errVarIsLocked,
			},
			{
				name:     "renewing lock after it has expired",
				varPath:  lockedVar.Path,
				lockID:   lockedVar.LockID(),
				expError: errVarIsLocked,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {

				rnReq := structs.VariablesRenewLockRequest{
					Path:   tc.varPath,
					LockID: tc.lockID,
					WriteRequest: structs.WriteRequest{
						Region:    "global",
						Namespace: structs.DefaultNamespace,
					},
				}

				rnResp := new(structs.VariablesApplyResponse)
				err := msgpackrpc.CallWithCodec(codec, structs.VariablesRenewLockRPCMethod, &rnReq, rnResp)
				must.ErrorContains(t, err, tc.expError.Error())
			})
		}
	})

	t.Run("successfully renewing lock", func(t *testing.T) {
		// Add a running timer for the lock so it is consider active and can be renewed.
		srv.lockTTLTimer.Create(vlResp.WrittenSVMeta.LockID(), 30*time.Second, func() {})

		rnReq := structs.VariablesRenewLockRequest{
			Path:   lockedVar.Path,
			LockID: lockedVar.LockID(),
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}

		rnResp := new(structs.VariablesApplyResponse)
		err := msgpackrpc.CallWithCodec(codec, structs.VariablesRenewLockRPCMethod, &rnReq, rnResp)
		must.NoError(t, err)
	})
}

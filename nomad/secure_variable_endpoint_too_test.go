package nomad

import (
	"fmt"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestSecureVariablesEndpoint_Apply_ACL(t *testing.T) {
	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	state := srv.fsm.State()
	// var cLock sync.Mutex
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
		"set":        structs.SVSet,
		"cas":        structs.SVCAS,
		"delete":     structs.SVDelete,
		"delete-cas": structs.SVDeleteCAS,
	}

	for name, op := range opMap {
		t.Run(name+"/no token", func(t *testing.T) {
			sv1 := sv1
			applyReq := structs.SVRequest{
				Op:           op,
				Var:          sv1,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			applyResp := new(structs.SVResponse)

			// cLock.Lock()
			fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
			err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, applyResp)
			// cLock.Unlock()
			require.EqualError(t, err, structs.ErrPermissionDenied.Error())
		})
	}

	t.Run("cas/management token/new", func(t *testing.T) {
		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultOk, applyResp.Result)
		require.Equal(t, fmt.Sprint(sv1.Items), fmt.Sprint(applyResp.Output.Items))

		svHold = applyResp.Output
	})

	t.Run("cas with current", func(t *testing.T) {
		sv := svHold
		sv.Items["new"] = "newVal"

		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)
		applyReq.AuthToken = rootToken.SecretID

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultOk, applyResp.Result)
		require.Equal(t, fmt.Sprint(sv.Items), fmt.Sprint(applyResp.Output.Items))

		svHold = applyResp.Output
	})

	t.Run("cas with stale", func(t *testing.T) {
		sv1 := sv1
		svHold := svHold

		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv1,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)
		applyReq.AuthToken = rootToken.SecretID

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultConflict, applyResp.Result)
		require.EqualValues(t, svHold.SecureVariableMetadata, applyResp.Conflict.SecureVariableMetadata)
		require.Equal(t, fmt.Sprint(svHold.Items), fmt.Sprint(applyResp.Conflict.Items))
	})

	sv3 := mock.SecureVariable()
	sv3.Path = "dropbox/a"
	sv3.ModifyIndex = 0

	t.Run("cas/write-only/read own new", func(t *testing.T) {
		sv3 := sv3
		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultOk, applyResp.Result)
		require.Equal(t, fmt.Sprint(sv3.Items), fmt.Sprint(applyResp.Output.Items))
		svHold = applyResp.Output
	})

	t.Run("cas/write only/conflict redacted", func(t *testing.T) {
		sv3 := sv3
		svHold := svHold

		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv3,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultRedacted, applyResp.Result)
		require.Equal(t, svHold.SecureVariableMetadata, applyResp.Conflict.SecureVariableMetadata)
		require.Nil(t, applyResp.Conflict.Items)
	})

	t.Run("cas/write only/read own upsert", func(t *testing.T) {
		sv := svHold
		sv.Items["upsert"] = "read"

		applyReq := structs.SVRequest{
			Op:  structs.SVCAS,
			Var: sv,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: writeToken.SecretID,
			},
		}
		applyResp := new(structs.SVResponse)

		// cLock.Lock()
		fmt.Printf("---called %q. Path: %s ModifyIndex %v\n ", applyReq.Op, applyReq.Var.Path, applyReq.Var.ModifyIndex)
		err := msgpackrpc.CallWithCodec(codec, structs.SecureVariablesApplyRPCMethod, &applyReq, &applyResp)
		// cLock.Unlock()

		require.NoError(t, err)
		require.Equal(t, structs.SVOpResultOk, applyResp.Result)
		require.Equal(t, fmt.Sprint(sv.Items), fmt.Sprint(applyResp.Output.Items))
	})
}

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var (
	cb = func(c *Config) {
		var ns int
		ns = 0
		c.LogLevel = "ERROR"
		c.Server.NumSchedulers = &ns
	}
)

func TestHTTP_SecureVariables(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, cb, func(s *TestAgent) {
		// These tests are run against the same running server in order to reduce
		// the costs of server startup and allow as much parallelization as possible
		// given the port reuse issue that we have seen with the current freeport
		t.Run("error_badverb_list", func(t *testing.T) {
			req, err := http.NewRequest("LOLWUT", "/v1/vars", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			_, err = s.Server.SecureVariablesListRequest(respW, req)
			require.EqualError(t, err, ErrInvalidMethod)
		})
		t.Run("error_parse_list", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/v1/vars?wait=99a", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			_, _ = s.Server.SecureVariablesListRequest(respW, req)
			require.Equal(t, http.StatusBadRequest, respW.Code)
			require.Equal(t, "Invalid wait time", string(respW.Body.Bytes()))
		})
		t.Run("error_rpc_list", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/v1/vars?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariablesListRequest(respW, req)
			require.EqualError(t, err, "No path to region")
			require.Nil(t, obj)
		})
		t.Run("list", func(t *testing.T) {
			// Test the empty list case
			req, err := http.NewRequest("GET", "/v1/vars", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariablesListRequest(respW, req)
			require.NoError(t, err)

			// add vars and test a populated backend
			svMap := mock.SecureVariables(4, 4)
			svs := svMap.List()
			svs[3].Path = svs[0].Path + "/child"
			for _, sv := range svs {
				require.NoError(t, rpcWriteSV(s, sv))
			}

			// Make the HTTP request
			req, err = http.NewRequest("GET", "/v1/vars", nil)
			require.NoError(t, err)
			respW = httptest.NewRecorder()

			// Make the request
			obj, err = s.Server.SecureVariablesListRequest(respW, req)
			require.NoError(t, err)

			// Check for the index
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			require.Equal(t, "true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-LastContact"))

			// Check the output (the 3 we register )
			require.Len(t, obj.([]*structs.SecureVariableMetadata), 4)

			// test prefix query
			req, err = http.NewRequest("GET", "/v1/vars?prefix="+svs[0].Path, nil)
			require.NoError(t, err)
			respW = httptest.NewRecorder()

			// Make the request
			obj, err = s.Server.SecureVariablesListRequest(respW, req)
			require.NoError(t, err)
			require.Len(t, obj.([]*structs.SecureVariableMetadata), 2)
		})
		rpcResetSV(s)

		t.Run("error_badverb_query", func(t *testing.T) {
			req, err := http.NewRequest("LOLWUT", "/v1/var/does/not/exist", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, ErrInvalidMethod)
			require.Nil(t, obj)
		})
		t.Run("error_parse_query", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/v1/var/does/not/exist?wait=99a", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			_, _ = s.Server.SecureVariableSpecificRequest(respW, req)
			require.Equal(t, http.StatusBadRequest, respW.Code)
			require.Equal(t, "Invalid wait time", string(respW.Body.Bytes()))
		})
		t.Run("error_rpc_query", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/v1/var/does/not/exist?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "No path to region")
			require.Nil(t, obj)
		})
		t.Run("query_unset_path", func(t *testing.T) {
			// Make a request for a non-existing variable
			req, err := http.NewRequest("GET", "/v1/var/", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "missing secure variable path")
			require.Nil(t, obj)
		})
		t.Run("query_unset_variable", func(t *testing.T) {
			// Make a request for a non-existing variable
			req, err := http.NewRequest("GET", "/v1/var/not/real", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "secure variable not found")
			require.Nil(t, obj)
		})
		t.Run("query", func(t *testing.T) {
			// Use RPC to make a test variable
			sv1 := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv1))

			// Query a variable
			req, err := http.NewRequest("GET", "/v1/var/"+sv1.Path, nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.NoError(t, err)

			// Check for the index
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			require.Equal(t, "true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-LastContact"))

			// Check the output
			require.Equal(t, sv1.Path, obj.(*structs.SecureVariableDecrypted).Path)
		})
		rpcResetSV(s)

		sv1 := mock.SecureVariable()
		t.Run("error_parse_create", func(t *testing.T) {
			buf := encodeBrokenReq(&sv1)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "unexpected EOF")
			require.Nil(t, obj)
		})
		t.Run("error_rpc_create", func(t *testing.T) {
			buf := encodeReq(sv1)
			req, err := http.NewRequest("PUT", "/v1/var/does/not/exist?region=bad", buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "No path to region")
			require.Nil(t, obj)
		})
		t.Run("create_no_items", func(t *testing.T) {
			sv2 := sv1.Copy()
			sv2.Items = nil
			buf := encodeReq(sv2)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "secure variable missing required Items object")
			require.Nil(t, obj)
		})
		t.Run("create", func(t *testing.T) {
			buf := encodeReq(sv1)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.NoError(t, err)
			require.Nil(t, obj)

			// Check for the index
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))

			// Check the variable was put
			out, err := rpcReadSV(s, sv1.Namespace, sv1.Path)
			require.NoError(t, err)
			require.NotNil(t, out)

			// fixup times and indexes so the equality check is less gross
			sv1.CreateIndex, sv1.ModifyIndex = out.CreateIndex, out.ModifyIndex
			sv1.CreateTime, sv1.ModifyTime = out.CreateTime, out.ModifyTime
			require.Equal(t, sv1.Path, out.Path)
			require.Equal(t, sv1, out)
		})
		rpcResetSV(s)

		t.Run("error_parse_update", func(t *testing.T) {
			sv1U := sv1.Copy()
			sv1U.Items["new"] = "new"

			// break the request body
			badBuf := encodeBrokenReq(&sv1U)

			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path, badBuf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "unexpected EOF")
			var cErr HTTPCodedError
			require.ErrorAs(t, err, &cErr)
			require.Equal(t, http.StatusBadRequest, cErr.Code())
			require.Nil(t, obj)
		})
		t.Run("error_rpc_update", func(t *testing.T) {
			sv1U := sv1.Copy()
			sv1U.Items["new"] = "new"

			// test broken rpc error
			buf := encodeReq(&sv1U)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path+"?region=bad", buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "No path to region")
			require.Nil(t, obj)
		})
		t.Run("update", func(t *testing.T) {
			sv := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv))
			sv, err := rpcReadSV(s, sv.Namespace, sv.Path)
			require.NoError(t, err)

			svU := sv.Copy()
			svU.Items["new"] = "new"
			// Make the HTTP request
			buf := encodeReq(&svU)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.NoError(t, err)
			require.Nil(t, obj)

			// Check for the index
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))

			{
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				require.NoError(t, err)
				require.NotNil(t, out)

				// Check that written varible does not equal the input to rule out input mutation
				require.NotEqual(t, svU.SecureVariableMetadata, out.SecureVariableMetadata)

				// Update the input token with the updated metadata so that we
				// can use a simple equality check
				svU.CreateIndex, svU.ModifyIndex = out.CreateIndex, out.ModifyIndex
				svU.CreateTime, svU.ModifyTime = out.CreateTime, out.ModifyTime
				require.Equal(t, svU.SecureVariableMetadata, out.SecureVariableMetadata)

				// fmt writes sorted output of maps for testability.
				require.Equal(t, fmt.Sprint(svU.Items), fmt.Sprint(out.Items))
			}
		})
		t.Run("update-cas", func(t *testing.T) {
			sv := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv))
			sv, err := rpcReadSV(s, sv.Namespace, sv.Path)
			require.NoError(t, err)

			svU := sv.Copy()
			svU.Items["new"] = "new"

			// Make the HTTP request
			{
				buf := encodeReq(&svU)
				req, err := http.NewRequest("PUT", "/v1/var/"+svU.Path+"?cas=1", buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Equal(t, http.StatusConflict, respW.Result().StatusCode)

				// Evaluate the conflict variable
				require.NotNil(t, obj)
				conflict, ok := obj.(*structs.SecureVariableDecrypted)
				require.True(t, ok, "Expected *structs.SecureVariableDecrypted, got %T", obj)
				require.True(t, sv.Equals(*conflict))

				// Check for the index
				require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			}
			// Check the variable was not updated
			{
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				require.NoError(t, err)
				require.Equal(t, sv, out)
			}
			// Make the HTTP request
			{
				buf := encodeReq(&svU)
				req, err := http.NewRequest("PUT", "/v1/var/"+svU.Path+"?cas="+fmt.Sprint(sv.ModifyIndex), buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Nil(t, obj)

				// Check for the index
				require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			}
			// Check the variable was created correctly
			{
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				require.NoError(t, err)
				require.NotNil(t, out)

				require.NotEqual(t, sv, out)
				require.NotEqual(t, svU.SecureVariableMetadata, out.SecureVariableMetadata)

				// Update the input token with the updated metadata so that we
				// can use a simple equality check
				svU.CreateIndex, svU.ModifyIndex = out.CreateIndex, out.ModifyIndex
				svU.CreateTime, svU.ModifyTime = out.CreateTime, out.ModifyTime
				require.Equal(t, svU.SecureVariableMetadata, out.SecureVariableMetadata)

				// fmt writes sorted output of maps for testability.
				require.Equal(t, fmt.Sprint(svU.Items), fmt.Sprint(out.Items))
			}
		})
		rpcResetSV(s)

		t.Run("error_rpc_delete", func(t *testing.T) {
			sv1 := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv1))

			// Make the HTTP request
			req, err := http.NewRequest("DELETE", "/v1/var/"+sv1.Path+"?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.EqualError(t, err, "No path to region")
			require.Nil(t, obj)
		})
		t.Run("delete-cas", func(t *testing.T) {
			sv := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv))
			sv, err := rpcReadSV(s, sv.Namespace, sv.Path)
			require.NoError(t, err)

			// Make the HTTP request
			{
				req, err := http.NewRequest("DELETE", "/v1/var/"+sv.Path+"?cas=1", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Equal(t, http.StatusConflict, respW.Result().StatusCode)

				// Evaluate the conflict variable
				require.NotNil(t, obj)
				conflict, ok := obj.(*structs.SecureVariableDecrypted)
				require.True(t, ok, "Expected *structs.SecureVariableDecrypted, got %T", obj)
				require.True(t, sv.Equals(*conflict))

				// Check for the index
				require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			}

			// Check variable was not deleted
			{
				svChk, err := rpcReadSV(s, sv.Namespace, sv.Path)
				require.NoError(t, err)
				require.NotNil(t, svChk)
				require.Equal(t, sv, svChk)
			}
			// Make the HTTP request
			{
				req, err := http.NewRequest("DELETE", "/v1/var/"+sv.Path+"?cas="+fmt.Sprint(sv.ModifyIndex), nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Nil(t, obj)
			}
			// Check variable was deleted
			{
				svChk, err := rpcReadSV(s, sv.Namespace, sv.Path)
				require.NoError(t, err)
				require.Nil(t, svChk)
			}
		})
		t.Run("delete", func(t *testing.T) {
			sv1 := mock.SecureVariable()
			require.NoError(t, rpcWriteSV(s, sv1))

			// Make the HTTP request
			req, err := http.NewRequest("DELETE", "/v1/var/"+sv1.Path, nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.SecureVariableSpecificRequest(respW, req)
			require.NoError(t, err)
			require.Nil(t, obj)

			// Check for the index
			require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
			require.Equal(t, http.StatusNoContent, respW.Result().StatusCode)

			// Check variable was deleted
			sv, err := rpcReadSV(s, sv1.Namespace, sv1.Path)
			require.NoError(t, err)
			require.Nil(t, sv)
		})
	})
}

// encodeBrokenReq is a test helper that damages input JSON in order to create
// a parsing error for testing error pathways.
func encodeBrokenReq(obj interface{}) io.ReadCloser {
	// var buf *bytes.Buffer
	// enc := json.NewEncoder(buf)
	// enc.Encode(obj)
	b, _ := json.Marshal(obj)
	b = b[0 : len(b)-5] // strip newline and final }
	return ioutil.NopCloser(bytes.NewReader(b))
}

// rpcReadSV lets this test read a secure variable using the RPC endpoint
func rpcReadSV(s *TestAgent, ns, p string) (*structs.SecureVariableDecrypted, error) {
	checkArgs := structs.SecureVariablesReadRequest{Path: p, QueryOptions: structs.QueryOptions{Namespace: ns, Region: "global"}}
	var checkResp structs.SecureVariablesReadResponse
	err := s.Agent.RPC(structs.SecureVariablesReadRPCMethod, &checkArgs, &checkResp)
	return checkResp.Data, err
}

// rpcWriteSV lets this test write a secure variable using the RPC endpoint
func rpcWriteSV(s *TestAgent, sv *structs.SecureVariableDecrypted) error {

	args := structs.SecureVariablesApplyRequest{
		Op:           structs.SVOpSet,
		Var:          sv,
		WriteRequest: structs.WriteRequest{Namespace: sv.Namespace, Region: "global"},
	}

	var resp structs.SecureVariablesApplyResponse
	err := s.Agent.RPC(structs.SecureVariablesApplyRPCMethod, &args, &resp)
	if err != nil {
		return err
	}
	nv, err := rpcReadSV(s, sv.Namespace, sv.Path)
	if err != nil {
		return err
	}
	sv.CreateIndex = nv.CreateIndex
	sv.CreateTime = nv.CreateTime
	sv.ModifyIndex = nv.ModifyIndex
	sv.ModifyTime = nv.ModifyTime
	return nil
}

// rpcResetSV lists all the secure variables for every namespace in the global
// region and deletes them using the RPC endpoints
func rpcResetSV(s *TestAgent) {
	var lArgs structs.SecureVariablesListRequest
	var lResp structs.SecureVariablesListResponse

	lArgs = structs.SecureVariablesListRequest{
		QueryOptions: structs.QueryOptions{
			Namespace: "*",
			Region:    "global",
		},
	}
	err := s.Agent.RPC(structs.SecureVariablesListRPCMethod, &lArgs, &lResp)
	require.NoError(s.T, err)

	dArgs := structs.SecureVariablesApplyRequest{
		Op:  structs.SVOpDelete,
		Var: &structs.SecureVariableDecrypted{},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	var dResp structs.SecureVariablesApplyResponse

	for _, v := range lResp.Data {
		dArgs.Var.Path = v.Path
		dArgs.Var.Namespace = v.Namespace
		err := s.Agent.RPC(structs.SecureVariablesApplyRPCMethod, &dArgs, &dResp)
		require.NoError(s.T, err)
	}

	err = s.Agent.RPC(structs.SecureVariablesListRPCMethod, &lArgs, &lResp)
	require.NoError(s.T, err)
	require.Equal(s.T, 0, len(lResp.Data))
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
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

func TestHTTP_Variables(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, cb, func(s *TestAgent) {
		// These tests are run against the same running server in order to reduce
		// the costs of server startup and allow as much parallelization as possible.
		t.Run("error_badverb_list", func(t *testing.T) {
			req, err := http.NewRequest("LOLWUT", "/v1/vars", nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()
			_, err = s.Server.VariablesListRequest(respW, req)
			must.ErrorContains(t, err, ErrInvalidMethod)
		})
		t.Run("error_parse_list", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/v1/vars?wait=99a", nil)
			require.NoError(t, err)

			respW := httptest.NewRecorder()
			_, _ = s.Server.VariablesListRequest(respW, req)
			must.Eq(t, http.StatusBadRequest, respW.Code)
			must.Eq(t, "Invalid wait time", string(respW.Body.Bytes()))
		})
		t.Run("error_rpc_list", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/v1/vars?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariablesListRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})
		t.Run("list", func(t *testing.T) {
			// Test the empty list case
			req, err := http.NewRequest(http.MethodGet, "/v1/vars", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariablesListRequest(respW, req)
			must.NoError(t, err)

			// add vars and test a populated backend
			svMap := mock.Variables(4, 4)
			svs := svMap.List()
			svs[3].Path = svs[0].Path + "/child"
			for _, sv := range svs {
				must.NoError(t, rpcWriteSV(s, sv, nil))
			}

			// Make the HTTP request
			req, err = http.NewRequest(http.MethodGet, "/v1/vars", nil)
			require.NoError(t, err)
			respW = httptest.NewRecorder()

			// Make the request
			obj, err = s.Server.VariablesListRequest(respW, req)
			must.NoError(t, err)

			// Check for the index

			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, "true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-LastContact")))

			// Check the output (the 4 we register )
			must.Len(t, 4, obj.([]*structs.VariableMetadata))

			// test prefix query
			req, err = http.NewRequest(http.MethodGet, "/v1/vars?prefix="+svs[0].Path, nil)
			require.NoError(t, err)
			respW = httptest.NewRecorder()

			// Make the request
			obj, err = s.Server.VariablesListRequest(respW, req)
			must.NoError(t, err)
			must.Len(t, 2, obj.([]*structs.VariableMetadata))
		})
		rpcResetSV(s)

		t.Run("error_badverb_query", func(t *testing.T) {
			req, err := http.NewRequest("LOLWUT", "/v1/var/does/not/exist", nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, ErrInvalidMethod)
			must.Nil(t, obj)
		})
		t.Run("error_parse_query", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/v1/var/does/not/exist?wait=99a", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			_, _ = s.Server.VariableSpecificRequest(respW, req)
			must.Eq(t, http.StatusBadRequest, respW.Code)
			must.Eq(t, "Invalid wait time", string(respW.Body.Bytes()))
		})
		t.Run("error_rpc_query", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/v1/var/does/not/exist?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})
		t.Run("query_unset_path", func(t *testing.T) {
			// Make a request for a non-existing variable
			req, err := http.NewRequest(http.MethodGet, "/v1/var/", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "missing variable path")
			must.Nil(t, obj)
		})
		t.Run("query_unset_variable", func(t *testing.T) {
			// Make a request for a non-existing variable
			req, err := http.NewRequest(http.MethodGet, "/v1/var/not/real", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "variable not found")
			must.Nil(t, obj)
		})
		t.Run("query", func(t *testing.T) {
			// Use RPC to make a test variable
			out := new(structs.VariableDecrypted)
			sv1 := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv1, out))

			// Query a variable
			req, err := http.NewRequest(http.MethodGet, "/v1/var/"+sv1.Path, nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, "true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-LastContact")))

			// Check the output
			must.Eq(t, out, obj.(*structs.VariableDecrypted))
		})
		rpcResetSV(s)

		sv1 := mock.Variable()
		t.Run("error_parse_create", func(t *testing.T) {
			buf := encodeBrokenReq(&sv1)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "unexpected EOF")
			must.Nil(t, obj)
		})
		t.Run("error_rpc_create", func(t *testing.T) {
			buf := encodeReq(sv1)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/does/not/exist?region=bad", buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})
		t.Run("create_no_items", func(t *testing.T) {
			sv2 := sv1.Copy()
			sv2.Items = nil
			buf := encodeReq(sv2)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "variable missing required Items object")
			must.Nil(t, obj)
		})
		t.Run("create", func(t *testing.T) {
			buf := encodeReq(sv1)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv1.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Test the returned object and rehydrate to a VariableDecrypted
			must.NotNil(t, obj)
			sv1, ok := obj.(*structs.VariableDecrypted)
			must.True(t, ok, must.Sprint(must.Sprint("Unable to convert obj to VariableDecrypted")))

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, fmt.Sprint(sv1.ModifyIndex), respW.HeaderMap.Get("X-Nomad-Index"))

			// Check the variable was put and that the returned item matched the
			// fetched value
			out, err := rpcReadSV(s, sv1.Namespace, sv1.Path)
			must.NoError(t, err)
			must.NotNil(t, out)
			must.Eq(t, sv1, out)
		})
		rpcResetSV(s)

		t.Run("error_parse_update", func(t *testing.T) {
			sv1U := sv1.Copy()
			sv1U.Items["new"] = "new"

			// break the request body
			badBuf := encodeBrokenReq(&sv1U)

			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv1.Path, badBuf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "unexpected EOF")

			var cErr HTTPCodedError
			if !errors.As(err, &cErr) {
				t.Fatalf("unexpected error")
			}
			must.Eq(t, http.StatusBadRequest, cErr.Code())
			must.Nil(t, obj)
		})
		t.Run("error_rpc_update", func(t *testing.T) {
			sv1U := sv1.Copy()
			sv1U.Items["new"] = "new"

			// test broken rpc error
			buf := encodeReq(&sv1U)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv1.Path+"?region=bad", buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})
		t.Run("update", func(t *testing.T) {
			sv := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv, sv))

			svU := sv.Copy()
			svU.Items["new"] = "new"
			// Make the HTTP request
			buf := encodeReq(&svU)
			req, err := http.NewRequest(http.MethodPut, "/v1/var/"+sv.Path, buf)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Test the returned object and rehydrate to a VariableDecrypted
			must.NotNil(t, obj)
			out, ok := obj.(*structs.VariableDecrypted)
			must.True(t, ok, must.Sprint("Unable to convert obj to VariableDecrypted"))

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, fmt.Sprint(out.ModifyIndex), respW.HeaderMap.Get("X-Nomad-Index"))

			{
				// Check that written varible does not equal the input to rule out input mutation
				must.NotEqual(t, svU.VariableMetadata, out.VariableMetadata)

				// Update the input token with the updated metadata so that we
				// can use a simple equality check
				svU.ModifyIndex = out.ModifyIndex
				svU.ModifyTime = out.ModifyTime
				must.Eq(t, &svU, out)
			}
		})

		t.Run("update_cas", func(t *testing.T) {
			sv := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv, sv))

			svU := sv.Copy()
			svU.Items["new"] = "new"

			// Make the HTTP request
			{
				buf := encodeReq(&svU)
				req, err := http.NewRequest(http.MethodPut, "/v1/var/"+svU.Path+"?cas=1", buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.VariableSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Eq(t, http.StatusConflict, respW.Result().StatusCode)

				// Evaluate the conflict variable
				must.NotNil(t, obj)
				conflict, ok := obj.(*structs.VariableDecrypted)
				must.True(t, ok, must.Sprintf("Expected *structs.VariableDecrypted, got %T", obj))
				must.Eq(t, conflict, sv)

				// Check for the index
				must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			}
			// Check the variable was not updated
			{
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				must.NoError(t, err)
				must.Eq(t, sv, out)
			}
			// Make the HTTP request
			{
				buf := encodeReq(&svU)
				req, err := http.NewRequest(http.MethodPut, "/v1/var/"+svU.Path+"?cas="+fmt.Sprint(sv.ModifyIndex), buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.VariableSpecificRequest(respW, req)
				must.NoError(t, err)

				// Test the returned object and rehydrate to a VariableDecrypted
				must.NotNil(t, obj)
				sv1, ok := obj.(*structs.VariableDecrypted)
				must.True(t, ok, must.Sprint("Unable to convert obj to VariableDecrypted"))

				// Check for the index
				must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
				must.Eq(t, fmt.Sprint(sv1.ModifyIndex), respW.HeaderMap.Get("X-Nomad-Index"))

				// Check the variable was put and that the returned item matched the
				// fetched value
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				must.NoError(t, err)
				must.NotNil(t, out)
				must.Eq(t, sv1, out)

			}
			// Check the variable was created correctly
			{
				out, err := rpcReadSV(s, sv.Namespace, sv.Path)
				must.NoError(t, err)
				must.NotNil(t, out)

				must.NotEq(t, sv, out)
				must.NotEqual(t, svU.VariableMetadata, out.VariableMetadata)

				// Update the input token with the updated metadata so that we
				// can use a simple equality check
				svU.CreateIndex, svU.ModifyIndex = out.CreateIndex, out.ModifyIndex
				svU.CreateTime, svU.ModifyTime = out.CreateTime, out.ModifyTime
				must.Eq(t, svU.VariableMetadata, out.VariableMetadata)

				// fmt writes sorted output of maps for testability.
				must.Eq(t, fmt.Sprint(svU.Items), fmt.Sprint(out.Items))
			}
		})

		t.Run("error_cas_and_acquire_lock", func(t *testing.T) {
			svLA := sv1.Copy()
			svLA.Items["new"] = "new"

			// break the request body
			badBuf := encodeBrokenReq(&svLA)

			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path+"?cas=1&"+acquireLockQueryParam, badBuf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "CAS can't be used with lock operations")

			var cErr HTTPCodedError
			if !errors.As(err, &cErr) {
				t.Fatalf("unexpected error")
			}

			must.Eq(t, http.StatusBadRequest, cErr.Code())
			must.Nil(t, obj)
		})
		t.Run("error_parse_acquire_lock", func(t *testing.T) {
			svLA := sv1.Copy()
			svLA.Items["new"] = "new"

			// break the request body
			badBuf := encodeBrokenReq(&svLA)

			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path+"?"+acquireLockQueryParam, badBuf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "unexpected EOF")
			var cErr HTTPCodedError

			if !errors.As(err, &cErr) {
				t.Fatalf("unexpected error")
			}

			must.Eq(t, http.StatusBadRequest, cErr.Code())
			must.Nil(t, obj)
		})
		t.Run("error_rpc_acquire_lock", func(t *testing.T) {
			svLA := sv1.Copy()
			svLA.Items["new"] = "new"

			// test broken rpc error
			buf := encodeReq(&svLA)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path+"?region=bad&"+acquireLockQueryParam, buf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})

		t.Run("acquire_lock", func(t *testing.T) {
			svLA := sv1

			svLA.Items["new"] = "new"
			// Make the HTTP request
			buf := encodeReq(&svLA)
			req, err := http.NewRequest("PUT", "/v1/var/"+svLA.Path+"?"+acquireLockQueryParam, buf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Test the returned object and rehydrate to a VariableDecrypted
			must.NotNil(t, obj)
			out, ok := obj.(*structs.VariableDecrypted)
			must.True(t, ok, must.Sprint("Unable to convert obj to VariableDecrypted"))

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, fmt.Sprint(out.ModifyIndex), respW.HeaderMap.Get("X-Nomad-Index"))

			// Check for the lock
			must.NotNil(t, out.VariableMetadata.Lock)
			must.NonZero(t, len(out.LockID()))

			// Check that written varible includes the new items
			must.Eq(t, svLA.Items, out.Items)

			// Update the lock information for the following tests
			sv1.VariableMetadata = out.VariableMetadata
		})

		t.Run("error_rpc_renew_lock", func(t *testing.T) {
			svRL := sv1.Copy()

			// test broken rpc error
			buf := encodeReq(&svRL)
			req, err := http.NewRequest("PUT", "/v1/var/"+sv1.Path+"?region=bad&"+renewLockQueryParam, buf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})

		t.Run("renew_lock", func(t *testing.T) {
			svRL := sv1.Copy()

			// Make the HTTP request
			buf := encodeReq(&svRL)
			req, err := http.NewRequest("PUT", "/v1/var/"+svRL.Path+"?"+renewLockQueryParam, buf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Test the returned object and rehydrate to a VariableDecrypted
			must.NotNil(t, obj)
			out, ok := obj.(*structs.VariableMetadata)
			must.True(t, ok, must.Sprint("Unable to convert obj to VariableDecrypted"))

			// Check for the lock
			must.NotNil(t, out.Lock)
			must.Eq(t, sv1.LockID(), out.Lock.ID)
		})

		t.Run("release_lock", func(t *testing.T) {
			svLR := *sv1

			svLR.Items = nil
			// Make the HTTP request
			buf := encodeReq(&svLR)

			req, err := http.NewRequest("PUT", "/v1/var/"+svLR.Path+"?"+releaseLockQueryParam, buf)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)

			// Test the returned object and rehydrate to a VariableDecrypted
			must.NotNil(t, obj)
			out, ok := obj.(*structs.VariableDecrypted)
			must.True(t, ok, must.Sprint("Unable to convert obj to VariableDecrypted"))

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, fmt.Sprint(out.ModifyIndex), respW.HeaderMap.Get("X-Nomad-Index"))

			// Check for the lock
			must.Nil(t, out.VariableMetadata.Lock)

			// Check that written variable is equal the input
			must.Zero(t, len(out.Items))

			// Remove the lock information from the mock variable for the following tests
			sv1.VariableMetadata = out.VariableMetadata
		})

		t.Run("error_rpc_delete", func(t *testing.T) {
			sv1 := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv1, nil))

			// Make the HTTP request
			req, err := http.NewRequest(http.MethodDelete, "/v1/var/"+sv1.Path+"?region=bad", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.ErrorContains(t, err, "No path to region")
			must.Nil(t, obj)
		})
		t.Run("delete-cas", func(t *testing.T) {
			sv := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv, nil))
			sv, err := rpcReadSV(s, sv.Namespace, sv.Path)
			must.NoError(t, err)

			// Make the HTTP request
			{
				req, err := http.NewRequest(http.MethodDelete, "/v1/var/"+sv.Path+"?cas=1", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.VariableSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Eq(t, http.StatusConflict, respW.Result().StatusCode)

				// Evaluate the conflict variable
				must.NotNil(t, obj)
				conflict, ok := obj.(*structs.VariableDecrypted)
				must.True(t, ok, must.Sprintf("Expected *structs.VariableDecrypted, got %T", obj))
				must.True(t, sv.Equal(*conflict))

				// Check for the index
				must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			}

			// Check variable was not deleted
			{
				svChk, err := rpcReadSV(s, sv.Namespace, sv.Path)
				must.NoError(t, err)
				must.NotNil(t, svChk)
				must.Eq(t, sv, svChk)
			}
			// Make the HTTP request
			{
				req, err := http.NewRequest(http.MethodDelete, "/v1/var/"+sv.Path+"?cas="+fmt.Sprint(sv.ModifyIndex), nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.VariableSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Nil(t, obj)
			}
			// Check variable was deleted
			{
				svChk, err := rpcReadSV(s, sv.Namespace, sv.Path)
				must.NoError(t, err)
				must.Nil(t, svChk)
			}
		})
		t.Run("delete", func(t *testing.T) {
			sv1 := mock.Variable()
			must.NoError(t, rpcWriteSV(s, sv1, nil))

			// Make the HTTP request
			req, err := http.NewRequest(http.MethodDelete, "/v1/var/"+sv1.Path, nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()

			// Make the request
			obj, err := s.Server.VariableSpecificRequest(respW, req)
			must.NoError(t, err)
			must.Nil(t, obj)

			// Check for the index
			must.NonZero(t, len(respW.HeaderMap.Get("X-Nomad-Index")))
			must.Eq(t, http.StatusNoContent, respW.Result().StatusCode)

			// Check variable was deleted
			sv, err := rpcReadSV(s, sv1.Namespace, sv1.Path)
			must.NoError(t, err)
			must.Nil(t, sv)
		})

		// WIP
		t.Run("error_parse_lock_acquire", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/v1/var/does/not/exist?wait=99a&lock=acquire", nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()
			_, _ = s.Server.VariableSpecificRequest(respW, req)
			must.Eq(t, http.StatusBadRequest, respW.Code)
			must.Eq(t, "Invalid wait time", string(respW.Body.Bytes()))
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
	return io.NopCloser(bytes.NewReader(b))
}

// rpcReadSV lets this test read a variable using the RPC endpoint
func rpcReadSV(s *TestAgent, ns, p string) (*structs.VariableDecrypted, error) {
	checkArgs := structs.VariablesReadRequest{Path: p, QueryOptions: structs.QueryOptions{Namespace: ns, Region: "global"}}
	var checkResp structs.VariablesReadResponse
	err := s.Agent.RPC(structs.VariablesReadRPCMethod, &checkArgs, &checkResp)
	return checkResp.Data, err
}

// rpcWriteSV lets this test write a variable using the RPC endpoint
func rpcWriteSV(s *TestAgent, sv, out *structs.VariableDecrypted) error {

	args := structs.VariablesApplyRequest{
		Op:           structs.VarOpSet,
		Var:          sv,
		WriteRequest: structs.WriteRequest{Namespace: sv.Namespace, Region: "global"},
	}

	var resp structs.VariablesApplyResponse
	err := s.Agent.RPC(structs.VariablesApplyRPCMethod, &args, &resp)
	if err != nil {
		return err
	}
	if out != nil {
		*out = *resp.Output
	}
	return nil
}

// rpcResetSV lists all the variables for every namespace in the global
// region and deletes them using the RPC endpoints
func rpcResetSV(s *TestAgent) {
	var lArgs structs.VariablesListRequest
	var lResp structs.VariablesListResponse

	lArgs = structs.VariablesListRequest{
		QueryOptions: structs.QueryOptions{
			Namespace: "*",
			Region:    "global",
		},
	}
	err := s.Agent.RPC(structs.VariablesListRPCMethod, &lArgs, &lResp)
	must.NoError(s.T, err)

	dArgs := structs.VariablesApplyRequest{
		Op:  structs.VarOpDelete,
		Var: &structs.VariableDecrypted{},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	var dResp structs.VariablesApplyResponse

	for _, v := range lResp.Data {
		dArgs.Var.Path = v.Path
		dArgs.Var.Namespace = v.Namespace
		err := s.Agent.RPC(structs.VariablesApplyRPCMethod, &dArgs, &dResp)
		must.NoError(s.T, err)
	}

	err = s.Agent.RPC(structs.VariablesListRPCMethod, &lArgs, &lResp)
	must.NoError(s.T, err)
	must.Eq(s.T, 0, len(lResp.Data))
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestHTTP_NodePool_List(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Populate state with test data.
		pool1 := mock.NodePool()
		pool2 := mock.NodePool()
		pool3 := mock.NodePool()
		args := structs.NodePoolUpsertRequest{
			NodePools: []*structs.NodePool{pool1, pool2, pool3},
		}
		var resp structs.GenericResponse
		err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
		must.NoError(t, err)

		// Make HTTP request.
		req, err := http.NewRequest("GET", "/v1/node/pools", nil)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		obj, err := s.Server.NodePoolsRequest(respW, req)
		must.NoError(t, err)

		// Expect 5 node pools: 3 created + 2 built-in.
		must.SliceLen(t, 5, obj.([]*structs.NodePool))

		// Verify response index.
		gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
		must.NoError(t, err)
		must.NonZero(t, gotIndex)
	})
}

func TestHTTP_NodePool_Info(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Populate state with test data.
		pool := mock.NodePool()
		args := structs.NodePoolUpsertRequest{
			NodePools: []*structs.NodePool{pool},
		}
		var resp structs.GenericResponse
		err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
		must.NoError(t, err)

		t.Run("test pool", func(t *testing.T) {
			// Make HTTP request for test pool.
			req, err := http.NewRequest("GET", fmt.Sprintf("/v1/node/pool/%s", pool.Name), nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.NodePoolSpecificRequest(respW, req)
			must.NoError(t, err)

			// Verify expected pool is returned.
			must.Eq(t, pool, obj.(*structs.NodePool), must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"CreateIndex",
				"ModifyIndex",
			)))

			// Verify response index.
			gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.NonZero(t, gotIndex)
		})

		t.Run("built-in pool", func(t *testing.T) {
			// Make HTTP request for built-in pool.
			req, err := http.NewRequest("GET", fmt.Sprintf("/v1/node/pool/%s", structs.NodePoolAll), nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.NodePoolSpecificRequest(respW, req)
			must.NoError(t, err)

			// Verify expected pool is returned.
			must.Eq(t, structs.NodePoolAll, obj.(*structs.NodePool).Name)

			// Verify response index.
			gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.NonZero(t, gotIndex)
		})

		t.Run("invalid pool", func(t *testing.T) {
			// Make HTTP request for built-in pool.
			req, err := http.NewRequest("GET", "/v1/node/pool/doesn-exist", nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			// Verify error.
			_, err = s.Server.NodePoolSpecificRequest(respW, req)
			must.ErrorContains(t, err, "not found")

			// Verify response index.
			gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.NonZero(t, gotIndex)
		})
	})
}

func TestHTTP_NodePool_Create(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create test node pool.
		pool := mock.NodePool()
		buf := encodeReq(pool)
		req, err := http.NewRequest("PUT", "/v1/node/pools", buf)
		must.NoError(t, err)

		respW := httptest.NewRecorder()
		obj, err := s.Server.NodePoolsRequest(respW, req)
		must.NoError(t, err)
		must.Nil(t, obj)

		// Verify response index.
		gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
		must.NoError(t, err)
		must.NonZero(t, gotIndex)

		// Verify test node pool is in state.
		got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
		must.NoError(t, err)
		must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
			structs.NodePool{},
			"CreateIndex",
			"ModifyIndex",
		)))
		must.Eq(t, gotIndex, got.CreateIndex)
		must.Eq(t, gotIndex, got.ModifyIndex)
	})
}

func TestHTTP_NodePool_Update(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		t.Run("success", func(t *testing.T) {
			// Populate state with test node pool.
			pool := mock.NodePool()
			args := structs.NodePoolUpsertRequest{
				NodePools: []*structs.NodePool{pool},
			}
			var resp structs.GenericResponse
			err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
			must.NoError(t, err)

			// Update node pool.
			updated := pool.Copy()
			updated.Description = "updated node pool"
			updated.Meta = map[string]string{
				"updated": "true",
			}
			updated.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
				SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			}

			buf := encodeReq(updated)
			req, err := http.NewRequest("PUT", fmt.Sprintf("/v1/node/pool/%s", updated.Name), buf)
			must.NoError(t, err)

			respW := httptest.NewRecorder()
			obj, err := s.Server.NodePoolsRequest(respW, req)
			must.NoError(t, err)
			must.Nil(t, obj)

			// Verify response index.
			gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.NonZero(t, gotIndex)

			// Verify node pool was updated.
			got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
			must.NoError(t, err)
			must.Eq(t, updated, got, must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"CreateIndex",
				"ModifyIndex",
			)))
			must.NotEq(t, gotIndex, got.CreateIndex)
			must.Eq(t, gotIndex, got.ModifyIndex)
		})

		t.Run("no name in path", func(t *testing.T) {
			// Populate state with test node pool.
			pool := mock.NodePool()
			args := structs.NodePoolUpsertRequest{
				NodePools: []*structs.NodePool{pool},
			}
			var resp structs.GenericResponse
			err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
			must.NoError(t, err)

			// Update node pool with no name in path.
			updated := pool.Copy()
			updated.Description = "updated node pool"
			updated.Meta = map[string]string{
				"updated": "true",
			}
			updated.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
				SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			}

			buf := encodeReq(updated)
			req, err := http.NewRequest("PUT", "/v1/node/pool/", buf)
			must.NoError(t, err)

			respW := httptest.NewRecorder()
			obj, err := s.Server.NodePoolsRequest(respW, req)
			must.NoError(t, err)
			must.Nil(t, obj)

			// Verify response index.
			gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.NonZero(t, gotIndex)

			// Verify node pool was updated.
			got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
			must.NoError(t, err)
			must.Eq(t, updated, got, must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"CreateIndex",
				"ModifyIndex",
			)))
		})

		t.Run("wrong name in path", func(t *testing.T) {
			// Populate state with test node pool.
			pool := mock.NodePool()
			args := structs.NodePoolUpsertRequest{
				NodePools: []*structs.NodePool{pool},
			}
			var resp structs.GenericResponse
			err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
			must.NoError(t, err)

			// Update node pool.
			updated := pool.Copy()
			updated.Description = "updated node pool"
			updated.Meta = map[string]string{
				"updated": "true",
			}
			updated.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
				SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			}

			// Make request with the wrong path.
			buf := encodeReq(updated)
			req, err := http.NewRequest("PUT", "/v1/node/pool/wrong", buf)
			must.NoError(t, err)

			respW := httptest.NewRecorder()
			_, err = s.Server.NodePoolSpecificRequest(respW, req)
			must.ErrorContains(t, err, "name does not match request path")

			// Verify node pool was NOT updated.
			got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
			must.NoError(t, err)
			must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"CreateIndex",
				"ModifyIndex",
			)))
		})
	})
}

func TestHTTP_NodePool_Delete(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Populate state with test node pool.
		pool := mock.NodePool()
		args := structs.NodePoolUpsertRequest{
			NodePools: []*structs.NodePool{pool},
		}
		var resp structs.GenericResponse
		err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
		must.NoError(t, err)

		// Delete test node pool.
		req, err := http.NewRequest("DELETE", fmt.Sprintf("/v1/node/pool/%s", pool.Name), nil)
		must.NoError(t, err)

		respW := httptest.NewRecorder()
		obj, err := s.Server.NodePoolSpecificRequest(respW, req)
		must.NoError(t, err)
		must.Nil(t, obj)

		// Verify node pool was deleted.
		got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
		must.NoError(t, err)
		must.Nil(t, got)
	})
}

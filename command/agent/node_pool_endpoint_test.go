// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
		req, err := http.NewRequest(http.MethodGet, "/v1/node/pools", nil)
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
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/node/pool/%s", pool.Name), nil)
			must.NoError(t, err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.NodePoolSpecificRequest(respW, req)
			must.NoError(t, err)

			// Verify expected pool is returned.
			must.Eq(t, pool, obj.(*structs.NodePool), must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"Hash",
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
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/node/pool/%s", structs.NodePoolAll), nil)
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
			req, err := http.NewRequest(http.MethodGet, "/v1/node/pool/doesn-exist", nil)
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
		req, err := http.NewRequest(http.MethodPut, "/v1/node/pools", buf)
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
			"Hash",
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

			buf := encodeReq(updated)
			req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/node/pool/%s", updated.Name), buf)
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
				"Hash",
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

			buf := encodeReq(updated)
			req, err := http.NewRequest(http.MethodPut, "/v1/node/pool/", buf)
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
				"Hash",
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

			// Make request with the wrong path.
			buf := encodeReq(updated)
			req, err := http.NewRequest(http.MethodPut, "/v1/node/pool/wrong", buf)
			must.NoError(t, err)

			respW := httptest.NewRecorder()
			_, err = s.Server.NodePoolSpecificRequest(respW, req)
			must.ErrorContains(t, err, "name does not match request path")

			// Verify node pool was NOT updated.
			got, err := s.Agent.server.State().NodePoolByName(nil, pool.Name)
			must.NoError(t, err)
			must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
				structs.NodePool{},
				"Hash",
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
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/node/pool/%s", pool.Name), nil)
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

func TestHTTP_NodePool_NodesList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t,
		func(c *Config) {
			// Disable client so it doesn't impact tests since we're registering
			// our own test nodes.
			c.Client.Enabled = false
		},
		func(s *TestAgent) {
			// Populate state with test data.
			pool1 := mock.NodePool()
			pool2 := mock.NodePool()
			args := structs.NodePoolUpsertRequest{
				NodePools: []*structs.NodePool{pool1, pool2},
			}
			var resp structs.GenericResponse
			err := s.Agent.RPC("NodePool.UpsertNodePools", &args, &resp)
			must.NoError(t, err)

			// Split test nodes between default, pool1, and pool2.
			nodesByPool := make(map[string][]*structs.Node)
			for i := 0; i < 10; i++ {
				node := mock.Node()
				switch i % 3 {
				case 0:
					// Leave node pool value empty so node goes to default.
				case 1:
					node.NodePool = pool1.Name
				case 2:
					node.NodePool = pool2.Name
				}
				nodeRegReq := structs.NodeRegisterRequest{
					Node: node,
					WriteRequest: structs.WriteRequest{
						Region: "global",
					},
				}
				var nodeRegResp structs.NodeUpdateResponse
				err := s.Agent.RPC("Node.Register", &nodeRegReq, &nodeRegResp)
				must.NoError(t, err)

				nodesByPool[node.NodePool] = append(nodesByPool[node.NodePool], node)
			}

			testCases := []struct {
				name          string
				pool          string
				args          string
				expectedNodes []*structs.Node
				expectedErr   string
				validateFn    func(*testing.T, []*structs.NodeListStub)
			}{
				{
					name:          "nodes in default",
					pool:          structs.NodePoolDefault,
					expectedNodes: nodesByPool[structs.NodePoolDefault],
					validateFn: func(t *testing.T, stubs []*structs.NodeListStub) {
						must.Nil(t, stubs[0].NodeResources)
					},
				},
				{
					name:          "nodes in pool1 with resources",
					pool:          pool1.Name,
					args:          "resources=true",
					expectedNodes: nodesByPool[pool1.Name],
					validateFn: func(t *testing.T, stubs []*structs.NodeListStub) {
						must.NotNil(t, stubs[0].NodeResources)
					},
				},
			}
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Make HTTP request.
					path := fmt.Sprintf("/v1/node/pool/%s/nodes?%s", tc.pool, tc.args)
					req, err := http.NewRequest(http.MethodGet, path, nil)
					must.NoError(t, err)
					respW := httptest.NewRecorder()

					obj, err := s.Server.NodePoolSpecificRequest(respW, req)
					if tc.expectedErr != "" {
						must.ErrorContains(t, err, tc.expectedErr)
						return
					}
					must.NoError(t, err)

					// Verify request only has expected nodes.
					stubs := obj.([]*structs.NodeListStub)
					must.Len(t, len(tc.expectedNodes), stubs)
					for _, node := range tc.expectedNodes {
						must.SliceContainsFunc(t, stubs, node, func(s *structs.NodeListStub, n *structs.Node) bool {
							return s.ID == n.ID
						})
					}

					// Verify respose.
					if tc.validateFn != nil {
						tc.validateFn(t, stubs)
					}

					// Verify response index.
					gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
					must.NoError(t, err)
					must.NonZero(t, gotIndex)
				})
			}
		})
}

func TestHTTP_NodePool_JobsList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {

		pool1, pool2 := mock.NodePool(), mock.NodePool()
		npUpReq := structs.NodePoolUpsertRequest{
			NodePools: []*structs.NodePool{pool1, pool2},
		}
		var npUpResp structs.GenericResponse
		err := s.Agent.RPC("NodePool.UpsertNodePools", &npUpReq, &npUpResp)
		must.NoError(t, err)

		for _, poolName := range []string{pool1.Name, "default", "all"} {
			for i := 0; i < 2; i++ {
				job := mock.MinJob()
				job.NodePool = poolName
				jobRegReq := structs.JobRegisterRequest{
					Job: job,
					WriteRequest: structs.WriteRequest{
						Region:    "global",
						Namespace: structs.DefaultNamespace,
					},
				}
				var jobRegResp structs.JobRegisterResponse
				must.NoError(t, s.Agent.RPC("Job.Register", &jobRegReq, &jobRegResp))
			}
		}

		// Make HTTP request to occupied pool
		req, err := http.NewRequest(http.MethodGet,
			fmt.Sprintf("/v1/node/pool/%s/jobs", pool1.Name), nil)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		obj, err := s.Server.NodePoolSpecificRequest(respW, req)
		must.NoError(t, err)
		must.SliceLen(t, 2, obj.([]*structs.JobListStub))

		// Verify response index.
		gotIndex, err := strconv.ParseUint(respW.HeaderMap.Get("X-Nomad-Index"), 10, 64)
		must.NoError(t, err)
		must.NonZero(t, gotIndex)

		// Make HTTP request to empty pool
		req, err = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/v1/node/pool/%s/jobs", pool2.Name), nil)
		must.NoError(t, err)
		respW = httptest.NewRecorder()

		obj, err = s.Server.NodePoolSpecificRequest(respW, req)
		must.NoError(t, err)
		must.SliceLen(t, 0, obj.([]*structs.JobListStub))

		// Make HTTP request to the "all"" pool
		req, err = http.NewRequest(http.MethodGet, "/v1/node/pool/all/jobs", nil)
		must.NoError(t, err)
		respW = httptest.NewRecorder()

		obj, err = s.Server.NodePoolSpecificRequest(respW, req)
		must.NoError(t, err)
		must.SliceLen(t, 2, obj.([]*structs.JobListStub))

	})
}

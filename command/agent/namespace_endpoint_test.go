// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_NamespaceList(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		ns1 := mock.Namespace()
		ns2 := mock.Namespace()
		ns3 := mock.Namespace()
		args := structs.NamespaceUpsertRequest{
			Namespaces:   []*structs.Namespace{ns1, ns2, ns3},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Namespace.UpsertNamespaces", &args, &resp))

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/namespaces", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NamespacesRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output (the 3 we register + default)
		assert.Len(obj.([]*structs.Namespace), 4)
	})
}

func TestHTTP_NamespaceQuery(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		ns1 := mock.Namespace()
		args := structs.NamespaceUpsertRequest{
			Namespaces:   []*structs.Namespace{ns1},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Namespace.UpsertNamespaces", &args, &resp))

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/namespace/"+ns1.Name, nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NamespaceSpecificRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output
		assert.Equal(ns1.Name, obj.(*structs.Namespace).Name)
	})
}

func TestHTTP_NamespaceCreate(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		ns1 := mock.Namespace()
		buf := encodeReq(ns1)
		req, err := http.NewRequest(http.MethodPut, "/v1/namespace", buf)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NamespaceCreateRequest(respW, req)
		assert.Nil(err)
		assert.Nil(obj)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		ns1.CreateIndex, ns1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(ns1.Name, out.Name)
		assert.Equal(ns1, out)
	})
}

func TestHTTP_NamespaceUpdate(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		ns1 := mock.Namespace()
		buf := encodeReq(ns1)
		req, err := http.NewRequest(http.MethodPut, "/v1/namespace/"+ns1.Name, buf)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NamespaceSpecificRequest(respW, req)
		assert.Nil(err)
		assert.Nil(obj)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		ns1.CreateIndex, ns1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(ns1.Name, out.Name)
		assert.Equal(ns1, out)
	})
}

func TestHTTP_NamespaceDelete(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		ns1 := mock.Namespace()
		args := structs.NamespaceUpsertRequest{
			Namespaces:   []*structs.Namespace{ns1},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Namespace.UpsertNamespaces", &args, &resp))

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodDelete, "/v1/namespace/"+ns1.Name, nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NamespaceSpecificRequest(respW, req)
		assert.Nil(err)
		assert.Nil(obj)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.Nil(out)
	})
}

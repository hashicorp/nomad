// +build ent

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_QuotaSpecAndUsages_List(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		qs1 := mock.QuotaSpec()
		qs2 := mock.QuotaSpec()
		qs3 := mock.QuotaSpec()
		args := structs.QuotaSpecUpsertRequest{
			Quotas:       []*structs.QuotaSpec{qs1, qs2, qs3},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Quota.UpsertQuotaSpecs", &args, &resp))

		// Make the HTTP request for quota specs
		req, err := http.NewRequest("GET", "/v1/quotas", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.QuotasRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output
		assert.Len(obj.([]*structs.QuotaSpec), 3)

		// Make the HTTP request for quota usages
		req, err = http.NewRequest("GET", "/v1/quota_usages", nil)
		assert.Nil(err)
		respW = httptest.NewRecorder()

		// Make the request
		obj, err = s.Server.QuotaUsagesRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output
		assert.Len(obj.([]*structs.QuotaUsage), 3)
	})
}

func TestHTTP_QuotaSpecAndUsage_Query(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		qs1 := mock.QuotaSpec()
		args := structs.QuotaSpecUpsertRequest{
			Quotas:       []*structs.QuotaSpec{qs1},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Quota.UpsertQuotaSpecs", &args, &resp))

		// Make the HTTP request for the quota spec
		req, err := http.NewRequest("GET", "/v1/quota/"+qs1.Name, nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.QuotaSpecificRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output
		assert.Equal(qs1.Name, obj.(*structs.QuotaSpec).Name)

		// Make the HTTP request for the quota usage
		req, err = http.NewRequest("GET", "/v1/quota/usage/"+qs1.Name, nil)
		assert.Nil(err)
		respW = httptest.NewRecorder()

		// Make the request
		obj, err = s.Server.QuotaSpecificRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"))
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"))

		// Check the output
		assert.Equal(qs1.Name, obj.(*structs.QuotaUsage).Name)
	})
}

func TestHTTP_QuotaSpec_Create(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		qs1 := mock.QuotaSpec()
		buf := encodeReq(qs1)
		req, err := http.NewRequest("PUT", "/v1/quota", buf)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.QuotaCreateRequest(respW, req)
		assert.Nil(err)
		assert.Nil(obj)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		qs1.CreateIndex, qs1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(qs1.Name, out.Name)
		assert.Equal(qs1, out)
	})
}

func TestHTTP_QuotaSpec_Update(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		qs1 := mock.QuotaSpec()
		buf := encodeReq(qs1)
		req, err := http.NewRequest("PUT", "/v1/quota/"+qs1.Name, buf)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.QuotaSpecificRequest(respW, req)
		assert.Nil(err)
		assert.Nil(obj)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		qs1.CreateIndex, qs1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(qs1.Name, out.Name)
		assert.Equal(qs1, out)
	})
}

func TestHTTP_QuotaSpec_Delete(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		qs1 := mock.QuotaSpec()
		args := structs.QuotaSpecUpsertRequest{
			Quotas:       []*structs.QuotaSpec{qs1},
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		assert.Nil(s.Agent.RPC("Quota.UpsertQuotaSpecs", &args, &resp))

		// Make the HTTP request
		req, err := http.NewRequest("DELETE", "/v1/quota/"+qs1.Name, nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.QuotaSpecificRequest(respW, req)
		assert.Nil(err)

		// Check for the index
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.Nil(out)
	})
}

// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) BatchJobQueueJobsRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	switch req.Method {
	case http.MethodGet:
		break
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var args structs.QueueJobsRequest
	var out structs.QueueJobsResponse

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}
	query := req.URL.Query()
	args.Sort = structs.SortOrder(query.Get("sort"))

	if err := s.agent.RPC("BatchJobQueue.Jobs", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

func (s *HTTPServer) BatchJobQueueTenantsRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	switch req.Method {
	case http.MethodGet:
		break
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var args structs.QueueTenantsRequest
	var out structs.QueueTenantsResponse

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	if err := s.agent.RPC("BatchJobQueue.Tenants", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

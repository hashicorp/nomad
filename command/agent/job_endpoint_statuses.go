// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

// JobStatusesRequest looks up the status of jobs' allocs and deployments,
// primarily for use in the UI on the /ui/jobs index page.
func (s *HTTPServer) JobStatusesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var out structs.JobStatusesResponse
	args := structs.JobStatusesRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	switch req.Method {
	case http.MethodGet, http.MethodPost:
		break
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	if includeChildren, err := parseBool(req, "include_children"); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	} else if includeChildren != nil {
		args.IncludeChildren = *includeChildren
	}

	// ostensibly GETs should not accept structured body, but the HTTP spec
	// on this is more what you'd call "guidelines" than actual rules.
	if req.Body != nil && req.Body != http.NoBody {
		var in api.JobStatusesRequest
		if err := decodeBody(req, &in); err != nil {
			return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("error decoding request: %v", err))
		}
		if len(in.Jobs) == 0 {
			return nil, CodedError(http.StatusBadRequest, "no jobs in request")
		}

		// each job has a separate namespace, so in case the NSes are mixed,
		// default to wildcard.
		// if all requested jobs turn out to have the same namespace,
		// then the RPC endpoint will notice that and override this anyway.
		if args.QueryOptions.Namespace == structs.DefaultNamespace {
			args.QueryOptions.Namespace = structs.AllNamespacesSentinel
		}

		args.Jobs = make([]structs.NamespacedID, len(in.Jobs))
		for i, j := range in.Jobs {
			if j.Namespace == "" {
				j.Namespace = structs.DefaultNamespace
			}
			args.Jobs[i] = structs.NamespacedID{
				ID:        j.ID,
				Namespace: j.Namespace,
			}
		}

		// not a direct assignment, because if it is false (default),
		// it could override the "include_children" query param.
		if in.IncludeChildren {
			args.IncludeChildren = true
		}
	}

	if err := s.agent.RPC("Job.Statuses", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Jobs, nil
}

package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) SecureVariablesListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.SecureVariablesListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SecureVariablesListResponse
	if err := s.agent.RPC(structs.SecureVariablesListRPCMethod, &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	if out.Data == nil {
		out.Data = make([]*structs.SecureVariableMetadata, 0)
	}
	return out.Data, nil
}

func (s *HTTPServer) SecureVariableSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/var/")
	if len(path) == 0 {
		return nil, CodedError(http.StatusBadRequest, "Missing secure variable path")
	}
	switch req.Method {
	case http.MethodGet:
		return s.secureVariableQuery(resp, req, path)
	case http.MethodPut, http.MethodPost:
		return s.secureVariableUpsert(resp, req, path)
	case http.MethodDelete:
		return s.secureVariableDelete(resp, req, path)
	default:
		return nil, CodedError(http.StatusBadRequest, ErrInvalidMethod)
	}
}

func (s *HTTPServer) secureVariableQuery(resp http.ResponseWriter, req *http.Request,
	path string) (interface{}, error) {
	args := structs.SecureVariablesReadRequest{
		Path: path,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}
	var out structs.SecureVariablesReadResponse
	if err := s.agent.RPC(structs.SecureVariablesReadRPCMethod, &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	if out.Data == nil {
		return nil, CodedError(http.StatusNotFound, "Secure variable not found")
	}
	return out.Data, nil
}

func (s *HTTPServer) secureVariableUpsert(resp http.ResponseWriter, req *http.Request,
	path string) (interface{}, error) {
	// Parse the SecureVariable
	var SecureVariable structs.SecureVariableDecrypted
	if err := decodeBody(req, &SecureVariable); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	if SecureVariable.Items == nil {
		return nil, CodedError(http.StatusBadRequest, "Secure variable missing required Items object.")
	}

	SecureVariable.Path = path

	args := structs.SecureVariablesUpsertRequest{
		Data: []*structs.SecureVariableDecrypted{&SecureVariable},
	}
	s.parseWriteRequest(req, &args.WriteRequest)
	if err := parseCAS(req, &args); err != nil {
		return nil, err
	}

	var out structs.SecureVariablesUpsertResponse
	if err := s.agent.RPC(structs.SecureVariablesUpsertRPCMethod, &args, &out); err != nil {
		if strings.Contains(err.Error(), "check-and-set conflict") {
			q, _ := s.secureVariableQuery(resp, req, path)
			sv := q.(*structs.SecureVariableDecrypted)
			out.Conflict = make([]*structs.SecureVariableDecrypted, 1)
			out.Conflict[0] = sv
			resp.WriteHeader(http.StatusConflict)
			return sv, nil
		}
		return nil, err
	}
	setIndex(resp, out.WriteMeta.Index)
	return nil, nil
}

func (s *HTTPServer) secureVariableDelete(resp http.ResponseWriter, req *http.Request,
	path string) (interface{}, error) {

	args := structs.SecureVariablesDeleteRequest{
		Path: path,
	}
	s.parseWriteRequest(req, &args.WriteRequest)
	if err := parseCAS(req, &args); err != nil {
		return nil, err
	}

	var out structs.SecureVariablesDeleteResponse
	if err := s.agent.RPC(structs.SecureVariablesDeleteRPCMethod, &args, &out); err != nil {
		if strings.HasPrefix(err.Error(), "check-and-set conflict") {
			resp.WriteHeader(http.StatusConflict)
		}
		return nil, err
	}
	setIndex(resp, out.WriteMeta.Index)
	resp.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func parseCAS(req *http.Request, rpc CheckIndexSetter) error {
	if cq := req.URL.Query().Get("cas"); cq != "" {
		ci, err := strconv.ParseUint(cq, 10, 64)
		if err != nil {
			return CodedError(http.StatusBadRequest, fmt.Sprintf("can not parse cas: %v", err))
		}
		rpc.SetCheckIndex(ci)
	}
	return nil
}

type CheckIndexSetter interface {
	SetCheckIndex(uint64)
}

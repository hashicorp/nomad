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
		return nil, CodedError(http.StatusBadRequest, "missing secure variable path")
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
		return nil, CodedError(http.StatusNotFound, "secure variable not found")
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
	if len(SecureVariable.Items) == 0 {
		return nil, CodedError(http.StatusBadRequest, "secure variable missing required Items object")
	}

	SecureVariable.Path = path

	args := structs.SecureVariablesApplyRequest{
		Op:  structs.SVOpSet,
		Var: &SecureVariable,
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	if isCas, checkIndex, err := parseCAS(req); err != nil {
		return nil, err
	} else if isCas {
		args.Op = structs.SVOpCAS
		args.Var.ModifyIndex = checkIndex
	}

	var out structs.SecureVariablesApplyResponse
	if err := s.agent.RPC(structs.SecureVariablesApplyRPCMethod, &args, &out); err != nil {

		// This handles the cases where there is an error in the CAS checking
		// function that renders it unable to return the conflicting variable
		// so it returns a text error. We can at least consider these unknown
		// moments to be CAS violations
		if strings.Contains(err.Error(), "cas error:") {
			resp.WriteHeader(http.StatusConflict)
		}

		// Otherwise it's a non-CAS error
		setIndex(resp, out.WriteMeta.Index)
		return nil, err
	}

	if out.Conflict != nil {
		setIndex(resp, out.Conflict.ModifyIndex)
		resp.WriteHeader(http.StatusConflict)
		return out.Conflict, nil
	}

	// Finally, we know that this is a success response, send it to the caller
	setIndex(resp, out.WriteMeta.Index)
	return nil, nil
}

func (s *HTTPServer) secureVariableDelete(resp http.ResponseWriter, req *http.Request,
	path string) (interface{}, error) {

	args := structs.SecureVariablesApplyRequest{
		Op: structs.SVOpDelete,
		Var: &structs.SecureVariableDecrypted{
			SecureVariableMetadata: structs.SecureVariableMetadata{
				Path: path,
			},
		},
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	if isCas, checkIndex, err := parseCAS(req); err != nil {
		return nil, err
	} else if isCas {
		args.Op = structs.SVOpDeleteCAS
		args.Var.ModifyIndex = checkIndex
	}

	var out structs.SecureVariablesApplyResponse
	if err := s.agent.RPC(structs.SecureVariablesApplyRPCMethod, &args, &out); err != nil {

		// This handles the cases where there is an error in the CAS checking
		// function that renders it unable to return the conflicting variable
		// so it returns a text error. We can at least consider these unknown
		// moments to be CAS violations
		if strings.HasPrefix(err.Error(), "cas error:") {
			resp.WriteHeader(http.StatusConflict)
		}
		setIndex(resp, out.WriteMeta.Index)
		return nil, err
	}

	// If the CAS validation can decode the conflicting value, Conflict is
	// non-Nil. Write out a 409 Conflict response.
	if out.Conflict != nil {
		setIndex(resp, out.Conflict.ModifyIndex)
		resp.WriteHeader(http.StatusConflict)
		return out.Conflict, nil
	}

	// Finally, we know that this is a success response, send it to the caller
	setIndex(resp, out.WriteMeta.Index)
	resp.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func parseCAS(req *http.Request) (bool, uint64, error) {
	if cq := req.URL.Query().Get("cas"); cq != "" {
		ci, err := strconv.ParseUint(cq, 10, 64)
		if err != nil {
			return true, 0, CodedError(http.StatusBadRequest, fmt.Sprintf("can not parse cas: %v", err))
		}
		return true, ci, nil
	}
	return false, 0, nil
}

type CheckIndexSetter interface {
	SetCheckIndex(uint64)
}

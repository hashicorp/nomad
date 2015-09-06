package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) EvalsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.EvalListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.EvalListResponse
	if err := s.agent.RPC("Eval.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Evaluations, nil
}

func (s *HTTPServer) EvalSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	evalID := strings.TrimPrefix(req.URL.Path, "/v1/evaluation/")
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.EvalSpecificRequest{
		EvalID: evalID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleEvalResponse
	if err := s.agent.RPC("Eval.GetEval", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Eval == nil {
		return nil, CodedError(404, "eval not found")
	}
	return out.Eval, nil
}

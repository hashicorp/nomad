// +build ent

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) QuotasRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.QuotaSpecListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.QuotaSpecListResponse
	if err := s.agent.RPC("Quota.ListQuotaSpecs", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Quotas == nil {
		out.Quotas = make([]*structs.QuotaSpec, 0)
	}
	return out.Quotas, nil
}

func (s *HTTPServer) QuotaUsagesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.QuotaUsageListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.QuotaUsageListResponse
	if err := s.agent.RPC("Quota.ListQuotaUsages", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Usages == nil {
		out.Usages = make([]*structs.QuotaUsage, 0)
	}
	return out.Usages, nil
}

func (s *HTTPServer) QuotaSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/quota/")
	switch {
	case strings.HasPrefix(path, "usage/"):
		quotaName := strings.TrimPrefix(path, "usage/")
		return s.quotaUsageQuery(resp, req, quotaName)
	default:
		return s.quotaSpecCRUD(resp, req, path)
	}
}

func (s *HTTPServer) quotaSpecCRUD(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	if len(name) == 0 {
		return nil, CodedError(400, "Missing quota name")
	}
	switch req.Method {
	case "GET":
		return s.quotaSpecQuery(resp, req, name)
	case "PUT", "POST":
		return s.quotaSpecUpdate(resp, req, name)
	case "DELETE":
		return s.quotaSpecDelete(resp, req, name)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) QuotaCreateRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" && req.Method != "POST" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	return s.quotaSpecUpdate(resp, req, "")
}

func (s *HTTPServer) quotaSpecQuery(resp http.ResponseWriter, req *http.Request,
	specName string) (interface{}, error) {
	args := structs.QuotaSpecSpecificRequest{
		Name: specName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleQuotaSpecResponse
	if err := s.agent.RPC("Quota.GetQuotaSpec", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Quota == nil {
		return nil, CodedError(404, "Quota not found")
	}
	return out.Quota, nil
}

func (s *HTTPServer) quotaUsageQuery(resp http.ResponseWriter, req *http.Request,
	specName string) (interface{}, error) {
	args := structs.QuotaUsageSpecificRequest{
		Name: specName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleQuotaUsageResponse
	if err := s.agent.RPC("Quota.GetQuotaUsage", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Usage == nil {
		return nil, CodedError(404, "Quota not found")
	}

	return out.Usage, nil
}

func (s *HTTPServer) quotaSpecUpdate(resp http.ResponseWriter, req *http.Request,
	specName string) (interface{}, error) {
	// Parse the quota spec
	var spec structs.QuotaSpec
	if err := decodeBody(req, &spec); err != nil {
		return nil, CodedError(500, err.Error())
	}

	// Ensure the spec name matches
	if specName != "" && spec.Name != specName {
		return nil, CodedError(400, "Quota name does not match request path")
	}

	// Format the request
	args := structs.QuotaSpecUpsertRequest{
		Quotas: []*structs.QuotaSpec{&spec},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Quota.UpsertQuotaSpecs", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) quotaSpecDelete(resp http.ResponseWriter, req *http.Request,
	specName string) (interface{}, error) {

	args := structs.QuotaSpecDeleteRequest{
		Names: []string{specName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Quota.DeleteQuotaSpecs", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// HostVolumeSpecificRequest dispatches GET and PUT
func (s *HTTPServer) HostVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Tokenize the suffix of the path to get the volume id
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/host/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	id := tokens[0]

	if len(tokens) == 1 {
		switch req.Method {
		case http.MethodGet:
			return s.hostVolumeGet(id, resp, req)
		case http.MethodPut, http.MethodPost:
			return s.hostVolumeRegister(resp, req)
		case http.MethodDelete:
			return nil, CodedError(http.StatusNotImplemented,
				"delete not implemented yet")
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	}

	if len(tokens) == 2 {
		switch req.Method {
		case http.MethodPut, http.MethodPost:
			if tokens[1] == "create" {
				return s.hostVolumeCreate(resp, req)
			}
		case http.MethodDelete:
			return nil, CodedError(http.StatusNotImplemented,
				"delete not implemented yet")
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) hostVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.HostVolumeGetRequest{
		ID: id,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.HostVolumeGetResponse
	if err := s.agent.RPC("HostVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}

	return out.Volume, nil
}

func (s *HTTPServer) hostVolumeRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.HostVolumeRegisterRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeRegisterResponse
	if err := s.agent.RPC("HostVolume.Register", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)

	return nil, nil
}

func (s *HTTPServer) hostVolumeCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.HostVolumeCreateRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeCreateResponse
	if err := s.agent.RPC("HostVolume.Create", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)

	return out, nil
}

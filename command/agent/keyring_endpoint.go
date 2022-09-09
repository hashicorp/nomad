package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// KeyringRequest is used route operator/raft API requests to the implementing
// functions.
func (s *HTTPServer) KeyringRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/keyring/")
	switch {
	case strings.HasPrefix(path, "keys"):
		if req.Method != http.MethodGet {
			return nil, CodedError(405, ErrInvalidMethod)
		}
		return s.keyringListRequest(resp, req)
	case strings.HasPrefix(path, "key"):
		keyID := strings.TrimPrefix(req.URL.Path, "/v1/operator/keyring/key/")
		switch req.Method {
		case http.MethodDelete:
			return s.keyringDeleteRequest(resp, req, keyID)
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	case strings.HasPrefix(path, "rotate"):
		switch req.Method {
		case http.MethodPost, http.MethodPut:
			return s.keyringRotateRequest(resp, req)
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) keyringListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.KeyringListRootKeyMetaRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.KeyringListRootKeyMetaResponse
	if err := s.agent.RPC("Keyring.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Keys == nil {
		out.Keys = make([]*structs.RootKeyMeta, 0)
	}
	return out.Keys, nil
}

func (s *HTTPServer) keyringRotateRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.KeyringRotateRootKeyRequest{}
	s.parseWriteRequest(req, &args.WriteRequest)

	query := req.URL.Query()
	switch query.Get("algo") {
	case string(structs.EncryptionAlgorithmAES256GCM):
		args.Algorithm = structs.EncryptionAlgorithmAES256GCM
	}

	if _, ok := query["full"]; ok {
		args.Full = true
	}

	var out structs.KeyringRotateRootKeyResponse
	if err := s.agent.RPC("Keyring.Rotate", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) keyringDeleteRequest(resp http.ResponseWriter, req *http.Request, keyID string) (interface{}, error) {

	args := structs.KeyringDeleteRootKeyRequest{KeyID: keyID}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.KeyringDeleteRootKeyResponse
	if err := s.agent.RPC("Keyring.Delete", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

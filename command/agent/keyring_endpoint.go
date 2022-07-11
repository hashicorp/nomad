package agent

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

// KeyringRequest is used route operator/raft API requests to the implementing
// functions.
func (s *HTTPServer) KeyringRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/keyring/")
	switch {
	case strings.HasPrefix(path, "keys"):
		switch req.Method {
		case http.MethodGet:
			return s.keyringListRequest(resp, req)
		case http.MethodPost, http.MethodPut:
			return s.keyringUpsertRequest(resp, req)
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	case strings.HasPrefix(path, "key"):
		keyID := strings.TrimPrefix(req.URL.Path, "/v1/operator/keyring/key/")
		switch req.Method {
		case http.MethodDelete:
			return s.keyringDeleteRequest(resp, req, keyID)
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	case strings.HasPrefix(path, "rotate"):
		return s.keyringRotateRequest(resp, req)
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

func (s *HTTPServer) keyringUpsertRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	var key api.RootKey
	if err := decodeBody(req, &key); err != nil {
		return nil, CodedError(400, err.Error())
	}
	if key.Meta == nil {
		return nil, CodedError(400, "decoded key did not include metadata")
	}

	const keyLen = 32

	decodedKey := make([]byte, keyLen)
	_, err := base64.StdEncoding.Decode(decodedKey, []byte(key.Key)[:keyLen])
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("could not decode key: %v", err))
	}

	args := structs.KeyringUpdateRootKeyRequest{
		RootKey: &structs.RootKey{
			Key: decodedKey,
			Meta: &structs.RootKeyMeta{
				KeyID:     key.Meta.KeyID,
				Algorithm: structs.EncryptionAlgorithm(key.Meta.Algorithm),
				State:     structs.RootKeyState(key.Meta.State),
			},
		},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.KeyringUpdateRootKeyResponse
	if err := s.agent.RPC("Keyring.Update", &args, &out); err != nil {
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

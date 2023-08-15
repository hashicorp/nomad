// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// jwksMinMaxAge is the minimum amount of time the JWKS endpoint will instruct
// consumers to cache a response for.
const jwksMinMaxAge = 15 * time.Minute

// JWKSRequest is used to handle JWKS requests. JWKS stands for JSON Web Key
// Sets and returns the public keys used for signing workload identities. Third
// parties may use this endpoint to validate workload identities. Consumers
// should cache this endpoint, preferably until an unknown kid is encountered.
func (s *HTTPServer) JWKSRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.GenericRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var rpcReply structs.KeyringListPublicResponse
	if err := s.agent.RPC("Keyring.ListPublic", &args, &rpcReply); err != nil {
		return nil, err
	}
	setMeta(resp, &rpcReply.QueryMeta)

	// Key set will change after max(CreateTime) + RotationThreshold.
	var newestKey int64
	jwks := make([]jose.JSONWebKey, 0, len(rpcReply.PublicKeys))
	for _, pubKey := range rpcReply.PublicKeys {
		if pubKey.CreateTime > newestKey {
			newestKey = pubKey.CreateTime
		}

		jwk := jose.JSONWebKey{
			KeyID:     pubKey.KeyID,
			Algorithm: pubKey.Algorithm,
			Use:       pubKey.Use,
		}

		// Convert public key bytes to an ed25519 public key
		if k, err := pubKey.GetPublicKey(); err == nil {
			jwk.Key = k
		} else {
			s.logger.Warn("error getting public key. server is likely newer than client", "err", err)
			continue
		}

		jwks = append(jwks, jwk)
	}

	// Have nonzero create times and threshold so set a reasonable cache time.
	if newestKey > 0 && rpcReply.RotationThreshold > 0 {
		exp := time.Unix(0, newestKey).Add(rpcReply.RotationThreshold)
		maxAge := helper.ExpiryToRenewTime(exp, time.Now, jwksMinMaxAge)
		resp.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(maxAge.Seconds())))
	}

	out := &jose.JSONWebKeySet{
		Keys: jwks,
	}

	return out, nil
}

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

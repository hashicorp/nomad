// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

import (
	"net/http"
)

// registerEnterpriseHandlers is a no-op for the oss release
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.entOnly))

	s.mux.HandleFunc("/v1/quotas", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota-usages", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota/", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota", s.wrap(s.entOnly))

	s.mux.HandleFunc("/v1/recommendation", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/recommendations", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/recommendations/apply", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/recommendation/", s.wrap(s.entOnly))
}

func (s *HTTPServer) entOnly(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, CodedError(501, ErrEntOnly)
}

// auditHandler wraps the passed handlerFn
func (s *HTTPServer) auditHandler(h handlerFn) handlerFn {
	return h
}

// auditHTTPHandler wraps  the passed handlerByteFn
func (s *HTTPServer) auditNonJSONHandler(h handlerByteFn) handlerByteFn {
	return h
}

// auditHTTPHandler wraps the passed http.Handler
func (s *HTTPServer) auditHTTPHandler(h http.Handler) http.Handler {
	return h
}

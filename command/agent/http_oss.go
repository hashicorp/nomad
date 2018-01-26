// +build !pro,!ent

package agent

import "net/http"

// registerEnterpriseHandlers is a no-op for the oss release
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.mux.HandleFunc("/v1/namespaces", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/namespace", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/namespace/", s.wrap(s.entOnly))

	s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.entOnly))

	s.mux.HandleFunc("/v1/quotas", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota-usages", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota/", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota", s.wrap(s.entOnly))
}

func (s *HTTPServer) entOnly(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, CodedError(501, ErrEntOnly)
}

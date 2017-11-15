// +build pro

package agent

import "net/http"

// ErrPremiumOnly is returned when accessing a premium only endpoint
const ErrPremimumOnly = "Nomad Premium only endpoint"

// registerEnterpriseHandlers registers Nomad Pro endpoints
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.registerProHandlers()

	s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.entOnly))

	s.mux.HandleFunc("/v1/quotas", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota-usages", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota/", s.wrap(s.entOnly))
	s.mux.HandleFunc("/v1/quota", s.wrap(s.entOnly))
}

func (s *HTTPServer) entOnly(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, CodedError(501, ErrPremimumOnly)
}

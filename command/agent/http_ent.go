// +build ent

package agent

// registerEnterpriseHandlers registers Nomad Pro and Premium endpoints
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.registerProHandlers()
	s.registerEntHandlers()
}

// registerEntHandlers registers Nomad Premium endpoints
func (s *HTTPServer) registerEntHandlers() {
	s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.SentinelPoliciesRequest))
	s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.SentinelPolicySpecificRequest))
}

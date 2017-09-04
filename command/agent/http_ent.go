// +build ent

package agent

// registerEnterpriseHandlers registers Nomad Pro and Premium endpoints
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.registerProHandlers()
}

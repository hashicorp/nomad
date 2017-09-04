// +build pro

package agent

// registerEnterpriseHandlers registers Nomad Pro endpoints
func (s *HTTPServer) registerEnterpriseHandlers() {
	s.registerProHandlers()
}

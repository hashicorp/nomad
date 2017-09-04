// +build pro ent

package agent

// registerProHandlers registers the Nomad Pro endpoints
func (s *HTTPServer) registerProHandlers() {
	s.mux.HandleFunc("/v1/namespaces", s.wrap(s.NamespacesRequest))
	s.mux.HandleFunc("/v1/namespace", s.wrap(s.NamespaceCreateRequest))
	s.mux.HandleFunc("/v1/namespace/", s.wrap(s.NamespaceSpecificRequest))
}

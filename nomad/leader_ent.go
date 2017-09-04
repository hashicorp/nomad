// +build ent

package nomad

// establishEnterpriseLeadership is used to instantiate Nomad Pro and Premium
// systems upon acquiring leadership.
func (s *Server) establishEnterpriseLeadership() error {
	return s.establishProLeadership()
}

// revokeEnterpriseLeadership is used to disable Nomad Pro and Premium systems
// upon losing leadership.
func (s *Server) revokeEnterpriseLeadership() error {
	return s.revokeProLeadership()
}

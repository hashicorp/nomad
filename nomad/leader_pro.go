// +build pro

package nomad

// establishEnterpriseLeadership is used to instantiate Nomad Pro systems upon
// acquiring leadership.
func (s *Server) establishEnterpriseLeadership(stopCh chan struct{}) error {
	return s.establishProLeadership(stopCh)
}

// revokeEnterpriseLeadership is used to disable Nomad Pro systems upon
// losing leadership.
func (s *Server) revokeEnterpriseLeadership() error {
	return s.revokeProLeadership()
}

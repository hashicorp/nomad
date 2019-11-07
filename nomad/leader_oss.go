// +build !pro,!ent

package nomad

// establishEnterpriseLeadership is a no-op on OSS.
func (s *Server) establishEnterpriseLeadership(stopCh chan struct{}) error {
	return nil
}

// revokeEnterpriseLeadership is a no-op on OSS>
func (s *Server) revokeEnterpriseLeadership() error {
	return nil
}

// +build ent

package nomad

// EnterpriseEndpoints holds the set of enterprise only endpoints to register
type EnterpriseEndpoints struct {
	Namespace *Namespace
}

// NewEnterpriseEndpoints returns the set of Nomad Enterprise and Pro only
// endpoints.
func NewEnterpriseEndpoints(s *Server) *EnterpriseEndpoints {
	return &EnterpriseEndpoints{
		Namespace: &Namespace{s},
	}
}

// Register register the enterprise endpoints.
func (e *EnterpriseEndpoints) Register(s *Server) {
	s.rpcServer.Register(e.Namespace)
}

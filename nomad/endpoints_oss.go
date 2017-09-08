// +build !pro,!ent

package nomad

// EnterpriseEndpoints holds the set of enterprise only endpoints to register
type EnterpriseEndpoints struct{}

// NewEnterpriseEndpoints returns a stub of the enterprise endpoints since there
// are none in oss
func NewEnterpriseEndpoints(s *Server) *EnterpriseEndpoints {
	return &EnterpriseEndpoints{}
}

// Register is a no-op in oss.
func (e *EnterpriseEndpoints) Register(s *Server) {}

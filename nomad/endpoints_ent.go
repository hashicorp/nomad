// +build ent

package nomad

import "net/rpc"

// EnterpriseEndpoints holds the set of enterprise only endpoints to register
type EnterpriseEndpoints struct {
	Namespace *Namespace
	Quota     *Quota
	Sentinel  *Sentinel
}

// NewEnterpriseEndpoints returns the set of Nomad Enterprise and Pro only
// endpoints.
func NewEnterpriseEndpoints(s *Server) *EnterpriseEndpoints {
	return &EnterpriseEndpoints{
		Namespace: &Namespace{s},
		Quota:     &Quota{s},
		Sentinel:  &Sentinel{s},
	}
}

// Register register the enterprise endpoints.
func (e *EnterpriseEndpoints) Register(rpcServer *rpc.Server) {
	rpcServer.Register(e.Namespace)
	rpcServer.Register(e.Quota)
	rpcServer.Register(e.Sentinel)
}

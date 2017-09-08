// +build !pro,!ent

package nomad

type EnterpriseState struct{}

func (s *Server) setupEnterprise(config *Config) error {
	return nil
}

func (s *Server) startEnterpriseBackground() {}

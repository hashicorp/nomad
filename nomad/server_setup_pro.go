// +build pro

package nomad

type EnterpriseState struct{}

func (s *Server) setupEnterprise(config *Config) error {
	s.setupEnterpriseAutopilot(config)

	return nil
}

func (s *Server) startEnterpriseBackground() {
}

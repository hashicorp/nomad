// +build ent

package nomad

import "github.com/hashicorp/sentinel/sentinel"

type EnterpriseState struct {
	// sentinel is a shared instance of the policy engine
	sentinel *sentinel.Sentinel
}

// setupEnterprise is used for Enterprise specific setup
func (s *Server) setupEnterprise(config *Config) error {
	// Enable the standard lib by default
	stdlib := sentinel.StdImports()

	// Setup the sentinel configuration
	sentConf := &sentinel.Config{
		Imports: stdlib,
	}
	if config.SentinelConfig != nil {
		for _, sImport := range config.SentinelConfig.Imports {
			sentConf.Imports[sImport.Name] = &sentinel.Import{
				Path: sImport.Path,
				Args: sImport.Args,
			}
			s.logger.Printf("[DEBUG] nomad.sentinel: enabling import %q via %q", sImport.Name, sImport.Path)
		}
	}

	// Create the Sentinel instance based on the configuration
	s.sentinel = sentinel.New(sentConf)
	return nil
}

// startEnterpriseBackground starts the Enterprise specific workers
func (s *Server) startEnterpriseBackground() {
	// Garbage collect Sentinel policies if enabled
	if s.config.ACLEnabled {
		go s.gcSentinelPolicies(s.shutdownCh)
	}
}

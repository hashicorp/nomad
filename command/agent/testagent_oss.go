//go:build !ent
// +build !ent

package agent

const (
	// EnterpriseTestAgent is used to configure a TestAgent's Enterprise flag
	EnterpriseTestAgent = false
)

func defaultEnterpriseTestServerConfig(c *ServerConfig) {}

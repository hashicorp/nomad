package config

import (
	"time"

	consul "github.com/hashicorp/consul/api"
)

// ConsulConfig contains the configuration information necessary to
// communicate with a Consul Agent in order to:
//
// - Register services and their checks with Consul
//
// - Bootstrap this Nomad Client with the list of Nomad Servers registered
//   with Consul
//
// Both the Agent and the executor need to be able to import ConsulConfig.
type ConsulConfig struct {
	// ServerServiceName is the name of the service that Nomad uses to register
	// servers with Consul
	ServerServiceName string `mapstructure:"server_service_name"`

	// ClientServiceName is the name of the service that Nomad uses to register
	// clients with Consul
	ClientServiceName string `mapstructure:"client_service_name"`

	// AutoAdvertise determines if this Nomad Agent will advertise its
	// services via Consul.  When true, Nomad Agent will register
	// services with Consul.
	AutoAdvertise bool `mapstructure:"auto_advertise"`

	// Addr is the address of the local Consul agent
	Addr string `mapstructure:"address"`

	// Timeout is used by Consul HTTP Client
	Timeout time.Duration `mapstructure:"timeout"`

	// Token is used to provide a per-request ACL token. This options overrides
	// the agent's default token
	Token string `mapstructure:"token"`

	// Auth is the information to use for http access to Consul agent
	Auth string `mapstructure:"auth"`

	// EnableSSL sets the transport scheme to talk to the Consul agent as https
	EnableSSL bool `mapstructure:"ssl"`

	// VerifySSL enables or disables SSL verification when the transport scheme
	// for the consul api client is https
	VerifySSL bool `mapstructure:"verify_ssl"`

	// CAFile is the path to the ca certificate used for Consul communication
	CAFile string `mapstructure:"ca_file"`

	// CertFile is the path to the certificate for Consul communication
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is the path to the private key for Consul communication
	KeyFile string `mapstructure:"key_file"`

	// ServerAutoJoin enables Nomad servers to find peers by querying Consul and
	// joining them
	ServerAutoJoin bool `mapstructure:"server_auto_join"`

	// ClientAutoJoin enables Nomad servers to find addresses of Nomad servers
	// and register with them
	ClientAutoJoin bool `mapstructure:"client_auto_join"`
}

// Merge merges two Consul Configurations together.
func (a *ConsulConfig) Merge(b *ConsulConfig) *ConsulConfig {
	result := *a

	if b.ServerServiceName != "" {
		result.ServerServiceName = b.ServerServiceName
	}
	if b.ClientServiceName != "" {
		result.ClientServiceName = b.ClientServiceName
	}
	if b.AutoAdvertise {
		result.AutoAdvertise = true
	}
	if b.Addr != "" {
		result.Addr = b.Addr
	}
	if b.Timeout != 0 {
		result.Timeout = b.Timeout
	}
	if b.Token != "" {
		result.Token = b.Token
	}
	if b.Auth != "" {
		result.Auth = b.Auth
	}
	if b.EnableSSL {
		result.EnableSSL = true
	}
	if b.VerifySSL {
		result.VerifySSL = true
	}
	if b.CAFile != "" {
		result.CAFile = b.CAFile
	}
	if b.CertFile != "" {
		result.CertFile = b.CertFile
	}
	if b.KeyFile != "" {
		result.KeyFile = b.KeyFile
	}
	if b.ServerAutoJoin {
		result.ServerAutoJoin = true
	}
	if b.ClientAutoJoin {
		result.ClientAutoJoin = true
	}
	return &result
}

// ApiConfig() returns a usable Consul config that can be passed directly to
// hashicorp/consul/api.  NOTE: datacenter is not set
func (c *ConsulConfig) ApiConfig() (*consul.Config, error) {
	config := consul.DefaultConfig()
	if c.Addr != "" {
		config.Address = c.Addr
	}
	if c.Token != "" {
		config.Token = c.Token
	}

	if c.Timeout != 0 {
		config.HttpClient.Timeout = c.Timeout
	}

	return config, nil
}

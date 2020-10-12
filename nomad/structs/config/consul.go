package config

import (
	"net/http"
	"strings"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper"
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
	ServerServiceName string `hcl:"server_service_name"`

	// ServerHTTPCheckName is the name of the health check that Nomad uses
	// to register the server HTTP health check with Consul
	ServerHTTPCheckName string `hcl:"server_http_check_name"`

	// ServerSerfCheckName is the name of the health check that Nomad uses
	// to register the server Serf health check with Consul
	ServerSerfCheckName string `hcl:"server_serf_check_name"`

	// ServerRPCCheckName is the name of the health check that Nomad uses
	// to register the server RPC health check with Consul
	ServerRPCCheckName string `hcl:"server_rpc_check_name"`

	// ClientServiceName is the name of the service that Nomad uses to register
	// clients with Consul
	ClientServiceName string `hcl:"client_service_name"`

	// ClientHTTPCheckName is the name of the health check that Nomad uses
	// to register the client HTTP health check with Consul
	ClientHTTPCheckName string `hcl:"client_http_check_name"`

	// Tags are optional service tags that get registered with the service
	// in Consul
	Tags []string `hcl:"tags"`

	// AutoAdvertise determines if this Nomad Agent will advertise its
	// services via Consul.  When true, Nomad Agent will register
	// services with Consul.
	AutoAdvertise *bool `hcl:"auto_advertise"`

	// ChecksUseAdvertise specifies that Consul checks should use advertise
	// address instead of bind address
	ChecksUseAdvertise *bool `hcl:"checks_use_advertise"`

	// Addr is the HTTP endpoint address of the local Consul agent
	//
	// Uses Consul's default and env var.
	Addr string `hcl:"address"`

	// GRPCAddr is the gRPC endpoint address of the local Consul agent
	GRPCAddr string `hcl:"grpc_address"`

	// Timeout is used by Consul HTTP Client
	Timeout    time.Duration `hcl:"-"`
	TimeoutHCL string        `hcl:"timeout" json:"-"`

	// Token is used to provide a per-request ACL token. This options overrides
	// the agent's default token
	Token string `hcl:"token"`

	// AllowUnauthenticated allows users to submit jobs requiring Consul
	// Service Identity tokens without providing a Consul token proving they
	// have access to such policies.
	AllowUnauthenticated *bool `hcl:"allow_unauthenticated"`

	// Auth is the information to use for http access to Consul agent
	Auth string `hcl:"auth"`

	// EnableSSL sets the transport scheme to talk to the Consul agent as https
	//
	// Uses Consul's default and env var.
	EnableSSL *bool `hcl:"ssl"`

	// ShareSSL enables Consul Connect Native applications to use the TLS
	// configuration of the Nomad Client for establishing connections to Consul.
	//
	// Does not include sharing of ACL tokens.
	ShareSSL *bool `hcl:"share_ssl"`

	// VerifySSL enables or disables SSL verification when the transport scheme
	// for the consul api client is https
	//
	// Uses Consul's default and env var.
	VerifySSL *bool `hcl:"verify_ssl"`

	// CAFile is the path to the ca certificate used for Consul communication.
	//
	// Uses Consul's default and env var.
	CAFile string `hcl:"ca_file"`

	// CertFile is the path to the certificate for Consul communication
	CertFile string `hcl:"cert_file"`

	// KeyFile is the path to the private key for Consul communication
	KeyFile string `hcl:"key_file"`

	// ServerAutoJoin enables Nomad servers to find peers by querying Consul and
	// joining them
	ServerAutoJoin *bool `hcl:"server_auto_join"`

	// ClientAutoJoin enables Nomad servers to find addresses of Nomad servers
	// and register with them
	ClientAutoJoin *bool `hcl:"client_auto_join"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`

	// Namespace sets the Consul namespace used for all calls against the
	// Consul API. If this is unset, then Nomad does not specify a consul namespace.
	Namespace string `hcl:"namespace"`
}

// DefaultConsulConfig() returns the canonical defaults for the Nomad
// `consul` configuration. Uses Consul's default configuration which reads
// environment variables.
func DefaultConsulConfig() *ConsulConfig {
	def := consul.DefaultConfig()
	return &ConsulConfig{
		ServerServiceName:    "nomad",
		ServerHTTPCheckName:  "Nomad Server HTTP Check",
		ServerSerfCheckName:  "Nomad Server Serf Check",
		ServerRPCCheckName:   "Nomad Server RPC Check",
		ClientServiceName:    "nomad-client",
		ClientHTTPCheckName:  "Nomad Client HTTP Check",
		AutoAdvertise:        helper.BoolToPtr(true),
		ChecksUseAdvertise:   helper.BoolToPtr(false),
		ServerAutoJoin:       helper.BoolToPtr(true),
		ClientAutoJoin:       helper.BoolToPtr(true),
		AllowUnauthenticated: helper.BoolToPtr(true),
		Timeout:              5 * time.Second,

		// From Consul api package defaults
		Addr:      def.Address,
		EnableSSL: helper.BoolToPtr(def.Scheme == "https"),
		VerifySSL: helper.BoolToPtr(!def.TLSConfig.InsecureSkipVerify),
		CAFile:    def.TLSConfig.CAFile,
		Namespace: def.Namespace,
	}
}

// AllowsUnauthenticated returns whether the config allows unauthenticated
// creation of Consul Service Identity tokens for Consul Connect enabled Tasks.
//
// If allow_unauthenticated is false, the operator must provide a token on
// job submission (i.e. -consul-token or $CONSUL_HTTP_TOKEN).
func (c *ConsulConfig) AllowsUnauthenticated() bool {
	return c.AllowUnauthenticated != nil && *c.AllowUnauthenticated
}

// Merge merges two Consul Configurations together.
func (c *ConsulConfig) Merge(b *ConsulConfig) *ConsulConfig {
	result := c.Copy()

	if b.ServerServiceName != "" {
		result.ServerServiceName = b.ServerServiceName
	}
	if b.ServerHTTPCheckName != "" {
		result.ServerHTTPCheckName = b.ServerHTTPCheckName
	}
	if b.ServerSerfCheckName != "" {
		result.ServerSerfCheckName = b.ServerSerfCheckName
	}
	if b.ServerRPCCheckName != "" {
		result.ServerRPCCheckName = b.ServerRPCCheckName
	}
	if b.ClientServiceName != "" {
		result.ClientServiceName = b.ClientServiceName
	}
	if b.ClientHTTPCheckName != "" {
		result.ClientHTTPCheckName = b.ClientHTTPCheckName
	}
	result.Tags = append(result.Tags, b.Tags...)
	if b.AutoAdvertise != nil {
		result.AutoAdvertise = helper.BoolToPtr(*b.AutoAdvertise)
	}
	if b.Addr != "" {
		result.Addr = b.Addr
	}
	if b.GRPCAddr != "" {
		result.GRPCAddr = b.GRPCAddr
	}
	if b.Timeout != 0 {
		result.Timeout = b.Timeout
	}
	if b.TimeoutHCL != "" {
		result.TimeoutHCL = b.TimeoutHCL
	}
	if b.Token != "" {
		result.Token = b.Token
	}
	if b.Auth != "" {
		result.Auth = b.Auth
	}
	if b.EnableSSL != nil {
		result.EnableSSL = helper.BoolToPtr(*b.EnableSSL)
	}
	if b.VerifySSL != nil {
		result.VerifySSL = helper.BoolToPtr(*b.VerifySSL)
	}
	if b.ShareSSL != nil {
		result.ShareSSL = helper.BoolToPtr(*b.ShareSSL)
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
	if b.ServerAutoJoin != nil {
		result.ServerAutoJoin = helper.BoolToPtr(*b.ServerAutoJoin)
	}
	if b.ClientAutoJoin != nil {
		result.ClientAutoJoin = helper.BoolToPtr(*b.ClientAutoJoin)
	}
	if b.ChecksUseAdvertise != nil {
		result.ChecksUseAdvertise = helper.BoolToPtr(*b.ChecksUseAdvertise)
	}
	if b.AllowUnauthenticated != nil {
		result.AllowUnauthenticated = helper.BoolToPtr(*b.AllowUnauthenticated)
	}
	if b.Namespace != "" {
		result.Namespace = b.Namespace
	}
	return result
}

// ApiConfig returns a usable Consul config that can be passed directly to
// hashicorp/consul/api.  NOTE: datacenter is not set
func (c *ConsulConfig) ApiConfig() (*consul.Config, error) {
	// Get the default config from consul to reuse things like the default
	// http.Transport.
	config := consul.DefaultConfig()
	if c.Addr != "" {
		config.Address = c.Addr
	}
	if c.Token != "" {
		config.Token = c.Token
	}
	if c.Timeout != 0 {
		// Create a custom Client to set the timeout
		if config.HttpClient == nil {
			config.HttpClient = &http.Client{}
		}
		config.HttpClient.Timeout = c.Timeout
		config.HttpClient.Transport = config.Transport
	}
	if c.Auth != "" {
		var username, password string
		if strings.Contains(c.Auth, ":") {
			split := strings.SplitN(c.Auth, ":", 2)
			username = split[0]
			password = split[1]
		} else {
			username = c.Auth
		}

		config.HttpAuth = &consul.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
	if c.EnableSSL != nil && *c.EnableSSL {
		config.Scheme = "https"
		config.TLSConfig = consul.TLSConfig{
			Address:  config.Address,
			CAFile:   c.CAFile,
			CertFile: c.CertFile,
			KeyFile:  c.KeyFile,
		}
		if c.VerifySSL != nil {
			config.TLSConfig.InsecureSkipVerify = !*c.VerifySSL
		}
		tlsConfig, err := consul.SetupTLSConfig(&config.TLSConfig)
		if err != nil {
			return nil, err
		}
		config.Transport.TLSClientConfig = tlsConfig
	}
	if c.Namespace != "" {
		config.Namespace = c.Namespace
	}
	return config, nil
}

// Copy returns a copy of this Consul config.
func (c *ConsulConfig) Copy() *ConsulConfig {
	if c == nil {
		return nil
	}

	nc := new(ConsulConfig)
	*nc = *c

	// Copy the bools
	if nc.AutoAdvertise != nil {
		nc.AutoAdvertise = helper.BoolToPtr(*nc.AutoAdvertise)
	}
	if nc.ChecksUseAdvertise != nil {
		nc.ChecksUseAdvertise = helper.BoolToPtr(*nc.ChecksUseAdvertise)
	}
	if nc.EnableSSL != nil {
		nc.EnableSSL = helper.BoolToPtr(*nc.EnableSSL)
	}
	if nc.VerifySSL != nil {
		nc.VerifySSL = helper.BoolToPtr(*nc.VerifySSL)
	}
	if nc.ShareSSL != nil {
		nc.ShareSSL = helper.BoolToPtr(*nc.ShareSSL)
	}
	if nc.ServerAutoJoin != nil {
		nc.ServerAutoJoin = helper.BoolToPtr(*nc.ServerAutoJoin)
	}
	if nc.ClientAutoJoin != nil {
		nc.ClientAutoJoin = helper.BoolToPtr(*nc.ClientAutoJoin)
	}
	if nc.AllowUnauthenticated != nil {
		nc.AllowUnauthenticated = helper.BoolToPtr(*nc.AllowUnauthenticated)
	}

	return nc
}

package config

// TLSConfig provides TLS related configuration
type TLSConfig struct {

	// EnableHTTP enabled TLS for http traffic to the Nomad server and clients
	EnableHTTP bool `mapstructure:"http"`

	// EnableRPC enables TLS for RPC and Raft traffic to the Nomad servers
	EnableRPC bool `mapstructure:"rpc"`

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<region>.nomad
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool `mapstructure:"verify_server_hostname"`

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string `mapstructure:"ca_file"`

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string `mapstructure:"key_file"`

	// Verify connections to the HTTPS API
	VerifyHTTPSClient bool `mapstructure:"verify_https_client"`
}

// Merge is used to merge two TLS configs together
func (t *TLSConfig) Merge(b *TLSConfig) *TLSConfig {
	result := *t

	if b.EnableHTTP {
		result.EnableHTTP = true
	}
	if b.EnableRPC {
		result.EnableRPC = true
	}
	if b.VerifyServerHostname {
		result.VerifyServerHostname = true
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
	if b.VerifyHTTPSClient {
		result.VerifyHTTPSClient = true
	}
	return &result
}

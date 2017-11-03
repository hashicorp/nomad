package config

import (
	"crypto/tls"
	"fmt"
	"sync"
)

// TLSConfig provides TLS related configuration
type TLSConfig struct {
	configLock sync.Mutex

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

	// KeyLoader is a helper to dynamically reload TLS configuration
	KeyLoader *KeyLoader

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string `mapstructure:"key_file"`

	// RPCUpgradeMode should be enabled when a cluster is being upgraded
	// to TLS. Allows servers to accept both plaintext and TLS connections and
	// should only be a temporary state.
	RPCUpgradeMode bool `mapstructure:"rpc_upgrade_mode"`

	// Verify connections to the HTTPS API
	VerifyHTTPSClient bool `mapstructure:"verify_https_client"`
}

type KeyLoader struct {
	cacheLock   sync.Mutex
	Certificate *tls.Certificate
}

// LoadKeyPair reloads the TLS certificate based on the specified certificate
// and key file. If successful, stores the certificate for further use.
func (k *KeyLoader) LoadKeyPair(certFile, keyFile string) (*tls.Certificate, error) {
	// Allow downgrading
	if certFile == "" && keyFile == "" {
		k.Certificate = nil
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	k.cacheLock.Lock()
	defer k.cacheLock.Unlock()

	k.Certificate = &cert
	return k.Certificate, nil
}

func (k *KeyLoader) GetOutgoingCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return k.Certificate, nil
}

func (t *TLSConfig) GetKeyLoader() *KeyLoader {
	// If the keyloader has not yet been initialized, do it here
	if t.KeyLoader == nil {
		t.configLock.Lock()
		t.KeyLoader = &KeyLoader{}
		t.configLock.Unlock()
	}
	return t.KeyLoader
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

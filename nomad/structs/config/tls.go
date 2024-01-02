// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

// TLSConfig provides TLS related configuration
type TLSConfig struct {

	// EnableHTTP enabled TLS for http traffic to the Nomad server and clients
	EnableHTTP bool `hcl:"http"`

	// EnableRPC enables TLS for RPC and Raft traffic to the Nomad servers
	EnableRPC bool `hcl:"rpc"`

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<region>.nomad
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool `hcl:"verify_server_hostname"`

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string `hcl:"ca_file"`

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string `hcl:"cert_file"`

	// KeyLoader is a helper to dynamically reload TLS configuration
	KeyLoader *KeyLoader

	keyloaderLock sync.Mutex

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string `hcl:"key_file"`

	// RPCUpgradeMode should be enabled when a cluster is being upgraded
	// to TLS. Allows servers to accept both plaintext and TLS connections and
	// should only be a temporary state.
	RPCUpgradeMode bool `hcl:"rpc_upgrade_mode"`

	// Verify connections to the HTTPS API
	VerifyHTTPSClient bool `hcl:"verify_https_client"`

	// Checksum is a MD5 hash of the certificate CA File, Certificate file, and
	// key file.
	Checksum string

	// TLSCipherSuites are operator-defined ciphers to be used in Nomad TLS
	// connections
	TLSCipherSuites string `hcl:"tls_cipher_suites"`

	// TLSMinVersion is used to set the minimum TLS version used for TLS
	// connections. Should be either "tls10", "tls11", or "tls12".
	TLSMinVersion string `hcl:"tls_min_version"`

	// TLSPreferServerCipherSuites controls whether the server selects the
	// client's most preferred ciphersuite, or the server's most preferred
	// ciphersuite. If true then the server's preference, as expressed in
	// the order of elements in CipherSuites, is used.
	TLSPreferServerCipherSuites bool `hcl:"tls_prefer_server_cipher_suites"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

type KeyLoader struct {
	cacheLock   sync.Mutex
	certificate *tls.Certificate
}

// LoadKeyPair reloads the TLS certificate based on the specified certificate
// and key file. If successful, stores the certificate for further use.
func (k *KeyLoader) LoadKeyPair(certFile, keyFile string) (*tls.Certificate, error) {
	k.cacheLock.Lock()
	defer k.cacheLock.Unlock()

	// Allow downgrading
	if certFile == "" && keyFile == "" {
		k.certificate = nil
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	k.certificate = &cert
	return k.certificate, nil
}

func (k *KeyLoader) GetCertificate() *tls.Certificate {
	k.cacheLock.Lock()
	defer k.cacheLock.Unlock()
	return k.certificate
}

// GetOutgoingCertificate fetches the currently-loaded certificate when
// accepting a TLS connection. This currently does not consider information in
// the ClientHello and only returns the certificate that was last loaded.
func (k *KeyLoader) GetOutgoingCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	k.cacheLock.Lock()
	defer k.cacheLock.Unlock()
	return k.certificate, nil
}

// GetClientCertificate fetches the currently-loaded certificate when the Server
// requests a certificate from the caller. This currently does not consider
// information in the ClientHello and only returns the certificate that was last
// loaded.
func (k *KeyLoader) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	k.cacheLock.Lock()
	defer k.cacheLock.Unlock()
	return k.certificate, nil
}

// GetKeyLoader returns the keyloader for a TLSConfig object. If the keyloader
// has not been initialized, it will first do so.
func (t *TLSConfig) GetKeyLoader() *KeyLoader {
	t.keyloaderLock.Lock()
	defer t.keyloaderLock.Unlock()

	// If the keyloader has not yet been initialized, do it here
	if t.KeyLoader == nil {
		t.KeyLoader = &KeyLoader{}
	}
	return t.KeyLoader
}

// Copy copies the fields of TLSConfig to another TLSConfig object. Required as
// to not copy mutexes between objects.
func (t *TLSConfig) Copy() *TLSConfig {
	if t == nil {
		return t
	}

	new := &TLSConfig{}
	new.EnableHTTP = t.EnableHTTP
	new.EnableRPC = t.EnableRPC
	new.VerifyServerHostname = t.VerifyServerHostname
	new.CAFile = t.CAFile
	new.CertFile = t.CertFile

	// Shallow copy the key loader as its GetOutgoingCertificate method is what
	// is used by the HTTP server to retrieve the certificate. If we create a new
	// KeyLoader struct, the HTTP server will still be calling the old
	// GetOutgoingCertificate method.
	t.keyloaderLock.Lock()
	new.KeyLoader = t.KeyLoader
	t.keyloaderLock.Unlock()

	new.KeyFile = t.KeyFile
	new.RPCUpgradeMode = t.RPCUpgradeMode
	new.VerifyHTTPSClient = t.VerifyHTTPSClient

	new.TLSCipherSuites = t.TLSCipherSuites
	new.TLSMinVersion = t.TLSMinVersion

	new.TLSPreferServerCipherSuites = t.TLSPreferServerCipherSuites

	new.SetChecksum()

	return new
}

func (t *TLSConfig) IsEmpty() bool {
	if t == nil {
		return true
	}

	return !t.EnableHTTP &&
		!t.EnableRPC &&
		!t.VerifyServerHostname &&
		t.CAFile == "" &&
		t.CertFile == "" &&
		t.KeyFile == "" &&
		!t.VerifyHTTPSClient
}

// Merge is used to merge two TLS configs together
func (t *TLSConfig) Merge(b *TLSConfig) *TLSConfig {
	result := t.Copy()

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
	if b.RPCUpgradeMode {
		result.RPCUpgradeMode = true
	}
	if b.TLSCipherSuites != "" {
		result.TLSCipherSuites = b.TLSCipherSuites
	}
	if b.TLSMinVersion != "" {
		result.TLSMinVersion = b.TLSMinVersion
	}
	if b.TLSPreferServerCipherSuites {
		result.TLSPreferServerCipherSuites = true
	}
	return result
}

// CertificateInfoIsEqual compares the fields of two TLS configuration objects
// for the fields that are specific to configuring a TLS connection
// It is possible for either the calling TLSConfig to be nil, or the TLSConfig
// that it is being compared against, so we need to handle both places. See
// server.go Reload for example.
func (t *TLSConfig) CertificateInfoIsEqual(newConfig *TLSConfig) (bool, error) {
	if t == nil || newConfig == nil {
		return t == newConfig, nil
	}

	if t.IsEmpty() && newConfig.IsEmpty() {
		return true, nil
	} else if t.IsEmpty() || newConfig.IsEmpty() {
		return false, nil
	}

	// Set the checksum if it hasn't yet been set (this should happen when the
	// config is parsed but this provides safety in depth)
	if newConfig.Checksum == "" {
		err := newConfig.SetChecksum()
		if err != nil {
			return false, err
		}
	}

	if t.Checksum == "" {
		err := t.SetChecksum()
		if err != nil {
			return false, err
		}
	}

	return t.Checksum == newConfig.Checksum, nil
}

// SetChecksum generates and sets the checksum for a TLS configuration
func (t *TLSConfig) SetChecksum() error {
	newCertChecksum, err := createChecksumOfFiles(t.CAFile, t.CertFile, t.KeyFile)
	if err != nil {
		return err
	}

	t.Checksum = newCertChecksum
	return nil
}

func getFileChecksum(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func createChecksumOfFiles(inputs ...string) (string, error) {
	h := md5.New()

	for _, input := range inputs {
		checksum, err := getFileChecksum(input)
		if err != nil {
			return "", err
		}
		io.WriteString(h, checksum)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

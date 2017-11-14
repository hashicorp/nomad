package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/hashicorp/nomad/nomad/structs/config"
)

// RegionSpecificWrapper is used to invoke a static Region and turns a
// RegionWrapper into a Wrapper type.
func RegionSpecificWrapper(region string, tlsWrap RegionWrapper) Wrapper {
	if tlsWrap == nil {
		return nil
	}
	return func(conn net.Conn) (net.Conn, error) {
		return tlsWrap(region, conn)
	}
}

// RegionWrapper is a function that is used to wrap a non-TLS connection and
// returns an appropriate TLS connection or error. This takes a Region as an
// argument.
type RegionWrapper func(region string, conn net.Conn) (net.Conn, error)

// Wrapper wraps a connection and enables TLS on it.
type Wrapper func(conn net.Conn) (net.Conn, error)

// Config used to create tls.Config
type Config struct {
	// VerifyIncoming is used to verify the authenticity of incoming connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncoming bool

	// VerifyOutgoing is used to verify the authenticity of outgoing connections.
	// This means that TLS requests are used, and TCP requests are not made. TLS connections
	// must match a provided certificate authority. This is used to verify authenticity of
	// server nodes.
	VerifyOutgoing bool

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<datacenter>.<domain>.
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string

	// KeyLoader dynamically reloads TLS configuration.
	KeyLoader *config.KeyLoader
}

// AppendCA opens and parses the CA file and adds the certificates to
// the provided CertPool.
func (c *Config) AppendCA(pool *x509.CertPool) error {
	if c.CAFile == "" {
		return nil
	}

	// Read the file
	data, err := ioutil.ReadFile(c.CAFile)
	if err != nil {
		return fmt.Errorf("Failed to read CA file: %v", err)
	}

	if !pool.AppendCertsFromPEM(data) {
		return fmt.Errorf("Failed to parse any CA certificates")
	}

	return nil
}

// LoadKeyPair is used to open and parse a certificate and key file
func (c *Config) LoadKeyPair() (*tls.Certificate, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, nil
	}

	if c.KeyLoader == nil {
		return nil, fmt.Errorf("No Keyloader object to perform LoadKeyPair")
	}

	cert, err := c.KeyLoader.LoadKeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}
	return cert, err
}

// OutgoingTLSConfig generates a TLS configuration for outgoing
// requests. It will return a nil config if this configuration should
// not use TLS for outgoing connections. Provides a callback to
// fetch certificates, allowing for reloading on the fly.
func (c *Config) OutgoingTLSConfig() (*tls.Config, error) {
	// If VerifyServerHostname is true, that implies VerifyOutgoing
	if c.VerifyServerHostname {
		c.VerifyOutgoing = true
	}
	if !c.VerifyOutgoing {
		return nil, nil
	}
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		RootCAs:            x509.NewCertPool(),
		InsecureSkipVerify: true,
	}
	if c.VerifyServerHostname {
		tlsConfig.InsecureSkipVerify = false
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if c.VerifyOutgoing && c.CAFile == "" {
		return nil, fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Parse the CA cert if any
	err := c.AppendCA(tlsConfig.RootCAs)
	if err != nil {
		return nil, err
	}

	cert, err := c.LoadKeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.GetCertificate = c.KeyLoader.GetOutgoingCertificate
	}

	return tlsConfig, nil
}

// OutgoingTLSWrapper returns a a Wrapper based on the OutgoingTLS
// configuration. If hostname verification is on, the wrapper
// will properly generate the dynamic server name for verification.
func (c *Config) OutgoingTLSWrapper() (RegionWrapper, error) {
	// Get the TLS config
	tlsConfig, err := c.OutgoingTLSConfig()
	if err != nil {
		return nil, err
	}

	// Check if TLS is not enabled
	if tlsConfig == nil {
		return nil, nil
	}

	// Generate the wrapper based on hostname verification
	if c.VerifyServerHostname {
		wrapper := func(region string, conn net.Conn) (net.Conn, error) {
			conf := tlsConfig.Clone()
			conf.ServerName = "server." + region + ".nomad"
			return WrapTLSClient(conn, conf)
		}
		return wrapper, nil
	} else {
		wrapper := func(dc string, c net.Conn) (net.Conn, error) {
			return WrapTLSClient(c, tlsConfig)
		}
		return wrapper, nil
	}

}

// Wrap a net.Conn into a client tls connection, performing any
// additional verification as needed.
//
// As of go 1.3, crypto/tls only supports either doing no certificate
// verification, or doing full verification including of the peer's
// DNS name. For consul, we want to validate that the certificate is
// signed by a known CA, but because consul doesn't use DNS names for
// node names, we don't verify the certificate DNS names. Since go 1.3
// no longer supports this mode of operation, we have to do it
// manually.
func WrapTLSClient(conn net.Conn, tlsConfig *tls.Config) (net.Conn, error) {
	var err error
	var tlsConn *tls.Conn

	tlsConn = tls.Client(conn, tlsConfig)

	// If crypto/tls is doing verification, there's no need to do
	// our own.
	if tlsConfig.InsecureSkipVerify == false {
		return tlsConn, nil
	}

	if err = tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}

	// The following is lightly-modified from the doFullHandshake
	// method in crypto/tls's handshake_client.go.
	opts := x509.VerifyOptions{
		Roots:         tlsConfig.RootCAs,
		CurrentTime:   time.Now(),
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	for i, cert := range certs {
		if i == 0 {
			continue
		}
		opts.Intermediates.AddCert(cert)
	}

	_, err = certs[0].Verify(opts)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}

	return tlsConn, err
}

// IncomingTLSConfig generates a TLS configuration for incoming requests
func (c *Config) IncomingTLSConfig() (*tls.Config, error) {
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		ClientCAs:  x509.NewCertPool(),
		ClientAuth: tls.NoClientCert,
	}

	// Parse the CA cert if any
	err := c.AppendCA(tlsConfig.ClientCAs)
	if err != nil {
		return nil, err
	}

	// Add cert/key
	cert, err := c.LoadKeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.GetCertificate = c.KeyLoader.GetOutgoingCertificate
	}

	// Check if we require verification
	if c.VerifyIncoming {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		if c.CAFile == "" {
			return nil, fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if cert == nil {
			return nil, fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}
	}

	return tlsConfig, nil
}

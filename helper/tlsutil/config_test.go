// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package tlsutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/yamux"
	"github.com/shoenig/test/must"
)

const (
	// See README.md for documentation
	cacert        = "./testdata/nomad-agent-ca.pem"
	fooclientcert = "./testdata/regionFoo-client-nomad.pem"
	fooclientkey  = "./testdata/regionFoo-client-nomad-key.pem"
	fooservercert = "./testdata/regionFoo-server-nomad.pem"
	fooserverkey  = "./testdata/regionFoo-server-nomad-key.pem"
	badcert       = "./testdata/badRegion-client-bad.pem"
	badkey        = "./testdata/badRegion-client-bad-key.pem"
)

func TestConfig_AppendCA_None(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	must.NoError(t, err)
}

func TestConfig_AppendCA_Valid(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		CAFile: cacert,
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	must.NoError(t, err)
}

func TestConfig_AppendCA_Valid_MultipleCerts(t *testing.T) {
	ci.Parallel(t)

	certs := `
-----BEGIN CERTIFICATE-----
MIICMzCCAdqgAwIBAgIUNZ9L86Xp9EuDH0/qyAesh599LXQwCgYIKoZIzj0EAwIw
eDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNh
biBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwGA1UECxMFTm9tYWQx
GDAWBgNVBAMTD25vbWFkLmhhc2hpY29ycDAeFw0xNjExMTAxOTQ4MDBaFw0yMTEx
MDkxOTQ4MDBaMHgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYw
FAYDVQQHEw1TYW4gRnJhbmNpc2NvMRIwEAYDVQQKEwlIYXNoaUNvcnAxDjAMBgNV
BAsTBU5vbWFkMRgwFgYDVQQDEw9ub21hZC5oYXNoaWNvcnAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAARfJmTdHzYIMPD8SK+kj5Gc79fmpOcg6wnb4JNVwCqWw9O+
uNdZJZWSi4Q/4HojM5FTSBqYxNgSrmY/o3oQrCPlo0IwQDAOBgNVHQ8BAf8EBAMC
AQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUOjVq/BectnhcKn6EHUD4NJFm
/UAwCgYIKoZIzj0EAwIDRwAwRAIgTemDJGSGtcQPXLWKiQNw4SKO9wAPhn/WoKW4
Ln2ZUe8CIDsQswBQS7URbqnKYDye2Y4befJkr4fmhhmMQb2ex9A4
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIICNTCCAZagAwIBAgIRANjgoh5iVZI26+Hz/K65G0UwCgYIKoZIzj0EAwQwNjEb
MBkGA1UEChMSSGFzaGlDb3JwIFRyYWluaW5nMRcwFQYDVQQDEw5zZXJ2aWNlLmNv
bnN1bDAeFw0xODA4MjMxNzM0NTBaFw0xODA5MjIxNzM0NTBaMDYxGzAZBgNVBAoT
Ekhhc2hpQ29ycCBUcmFpbmluZzEXMBUGA1UEAxMOc2VydmljZS5jb25zdWwwgZsw
EAYHKoZIzj0CAQYFK4EEACMDgYYABAGjC4sWsOfirS/DQ9/e7PdQeJwlOjziiOx/
CALjS6ryEDkZPqRqMuoFXfudAmfdk6tl8AT1IKMVcgiQU5jkm7fliwFIk48uh+n2
obqZjwDyM76VYBVSYi6i3BPXown1ivIMJNQS1txnWZLZHsv+WxbHydS+GNOAwKDK
KsXj9dEhd36pvaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8w
HQYDVR0OBBYEFIk3oG2hu0FxueW4e7fL+FdMOquBMAoGCCqGSM49BAMEA4GMADCB
iAJCAPIPwPyk+8Ymj7Zlvb5qIUQg+UxoacAeJtFZrJ8xQjro0YjsM33O86rAfw+x
sWWGul4Ews93KFBXvhbKCwb0F0PhAkIAh2z7COsKcQzvBoIy+Kx92+9j/sUjlzzl
TttDu+g2VdbcBwVDZ49X2Md6OY2N3G8Irdlj+n+mCQJaHwVt52DRzz0=
-----END CERTIFICATE-----
`

	tmpCAFile, err := os.CreateTemp("/tmp", "test_ca_file")
	must.NoError(t, err)
	defer os.Remove(tmpCAFile.Name())

	_, err = tmpCAFile.Write([]byte(certs))
	must.NoError(t, err)
	tmpCAFile.Close()

	conf := &Config{
		CAFile: tmpCAFile.Name(),
	}
	pool := x509.NewCertPool()
	must.NoError(t, conf.AppendCA(pool))

	must.Len(t, 2, pool.Subjects())
}

// TestConfig_AppendCA_Valid_Whitespace asserts that a PEM file containing
// trailing whitespace is valid.
func TestConfig_AppendCA_Valid_Whitespace(t *testing.T) {
	ci.Parallel(t)

	const cacertWhitespace = "./testdata/whitespace-agent-ca.pem"
	conf := &Config{
		CAFile: cacertWhitespace,
	}
	pool := x509.NewCertPool()
	must.NoError(t, conf.AppendCA(pool))
	must.Len(t, 1, pool.Subjects())
}

// TestConfig_AppendCA_Invalid_MultipleCerts_Whitespace asserts that a PEM file
// containing non-PEM data between certificate blocks is still valid.
func TestConfig_AppendCA_Valid_MultipleCerts_ExtraData(t *testing.T) {
	ci.Parallel(t)

	certs := `
Did you know...
-----BEGIN CERTIFICATE-----
MIICMzCCAdqgAwIBAgIUNZ9L86Xp9EuDH0/qyAesh599LXQwCgYIKoZIzj0EAwIw
eDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNh
biBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwGA1UECxMFTm9tYWQx
GDAWBgNVBAMTD25vbWFkLmhhc2hpY29ycDAeFw0xNjExMTAxOTQ4MDBaFw0yMTEx
MDkxOTQ4MDBaMHgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYw
FAYDVQQHEw1TYW4gRnJhbmNpc2NvMRIwEAYDVQQKEwlIYXNoaUNvcnAxDjAMBgNV
BAsTBU5vbWFkMRgwFgYDVQQDEw9ub21hZC5oYXNoaWNvcnAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAARfJmTdHzYIMPD8SK+kj5Gc79fmpOcg6wnb4JNVwCqWw9O+
uNdZJZWSi4Q/4HojM5FTSBqYxNgSrmY/o3oQrCPlo0IwQDAOBgNVHQ8BAf8EBAMC
AQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUOjVq/BectnhcKn6EHUD4NJFm
/UAwCgYIKoZIzj0EAwIDRwAwRAIgTemDJGSGtcQPXLWKiQNw4SKO9wAPhn/WoKW4
Ln2ZUe8CIDsQswBQS7URbqnKYDye2Y4befJkr4fmhhmMQb2ex9A4
-----END CERTIFICATE-----

...PEM parsers don't care about data...

-----BEGIN CERTIFICATE-----
MIICNTCCAZagAwIBAgIRANjgoh5iVZI26+Hz/K65G0UwCgYIKoZIzj0EAwQwNjEb
MBkGA1UEChMSSGFzaGlDb3JwIFRyYWluaW5nMRcwFQYDVQQDEw5zZXJ2aWNlLmNv
bnN1bDAeFw0xODA4MjMxNzM0NTBaFw0xODA5MjIxNzM0NTBaMDYxGzAZBgNVBAoT
Ekhhc2hpQ29ycCBUcmFpbmluZzEXMBUGA1UEAxMOc2VydmljZS5jb25zdWwwgZsw
EAYHKoZIzj0CAQYFK4EEACMDgYYABAGjC4sWsOfirS/DQ9/e7PdQeJwlOjziiOx/
CALjS6ryEDkZPqRqMuoFXfudAmfdk6tl8AT1IKMVcgiQU5jkm7fliwFIk48uh+n2
obqZjwDyM76VYBVSYi6i3BPXown1ivIMJNQS1txnWZLZHsv+WxbHydS+GNOAwKDK
KsXj9dEhd36pvaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8w
HQYDVR0OBBYEFIk3oG2hu0FxueW4e7fL+FdMOquBMAoGCCqGSM49BAMEA4GMADCB
iAJCAPIPwPyk+8Ymj7Zlvb5qIUQg+UxoacAeJtFZrJ8xQjro0YjsM33O86rAfw+x
sWWGul4Ews93KFBXvhbKCwb0F0PhAkIAh2z7COsKcQzvBoIy+Kx92+9j/sUjlzzl
TttDu+g2VdbcBwVDZ49X2Md6OY2N3G8Irdlj+n+mCQJaHwVt52DRzz0=
-----END CERTIFICATE-----

...outside of -----XXX----- blocks?
`

	tmpCAFile, err := os.CreateTemp("/tmp", "test_ca_file_extra")
	must.NoError(t, err)
	defer os.Remove(tmpCAFile.Name())
	_, err = tmpCAFile.Write([]byte(certs))
	must.NoError(t, err)
	tmpCAFile.Close()

	conf := &Config{
		CAFile: tmpCAFile.Name(),
	}
	pool := x509.NewCertPool()
	err = conf.AppendCA(pool)

	must.NoError(t, err)
	must.Len(t, 2, pool.Subjects())
}

// TestConfig_AppendCA_Invalid_MultipleCerts asserts only the valid certificate
// is returned.
func TestConfig_AppendCA_Invalid_MultipleCerts(t *testing.T) {
	ci.Parallel(t)

	certs := `
-----BEGIN CERTIFICATE-----
MIICMzCCAdqgAwIBAgIUNZ9L86Xp9EuDH0/qyAesh599LXQwCgYIKoZIzj0EAwIw
eDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNh
biBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwGA1UECxMFTm9tYWQx
GDAWBgNVBAMTD25vbWFkLmhhc2hpY29ycDAeFw0xNjExMTAxOTQ4MDBaFw0yMTEx
MDkxOTQ4MDBaMHgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYw
FAYDVQQHEw1TYW4gRnJhbmNpc2NvMRIwEAYDVQQKEwlIYXNoaUNvcnAxDjAMBgNV
BAsTBU5vbWFkMRgwFgYDVQQDEw9ub21hZC5oYXNoaWNvcnAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAARfJmTdHzYIMPD8SK+kj5Gc79fmpOcg6wnb4JNVwCqWw9O+
uNdZJZWSi4Q/4HojM5FTSBqYxNgSrmY/o3oQrCPlo0IwQDAOBgNVHQ8BAf8EBAMC
AQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUOjVq/BectnhcKn6EHUD4NJFm
/UAwCgYIKoZIzj0EAwIDRwAwRAIgTemDJGSGtcQPXLWKiQNw4SKO9wAPhn/WoKW4
Ln2ZUe8CIDsQswBQS7URbqnKYDye2Y4befJkr4fmhhmMQb2ex9A4
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
Invalid
-----END CERTIFICATE-----`

	tmpCAFile, err := os.CreateTemp("/tmp", "test_ca_file")
	must.NoError(t, err)
	defer os.Remove(tmpCAFile.Name())
	_, err = tmpCAFile.Write([]byte(certs))
	must.NoError(t, err)
	tmpCAFile.Close()

	conf := &Config{
		CAFile: tmpCAFile.Name(),
	}
	pool := x509.NewCertPool()
	must.NoError(t, conf.AppendCA(pool))
	must.Len(t, 1, pool.Subjects())
}

func TestConfig_AppendCA_Invalid(t *testing.T) {
	ci.Parallel(t)

	{
		conf := &Config{
			CAFile: "invalidFile",
		}
		pool := x509.NewCertPool()
		err := conf.AppendCA(pool)
		must.ErrorContains(t, err, "Failed to read CA file")
		must.Len(t, 0, pool.Subjects())
	}

	{
		tmpFile, err := os.CreateTemp("/tmp", "test_ca_file")
		must.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write([]byte("Invalid CA Content!"))
		must.NoError(t, err)

		conf := &Config{
			CAFile: tmpFile.Name(),
		}
		pool := x509.NewCertPool()
		err = conf.AppendCA(pool)
		must.ErrorContains(t, err, "Failed to parse any valid certificates in CA file:")
		must.Len(t, 0, pool.Subjects())
	}
}

func TestConfig_CACertificate_Valid(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		CAFile: cacert,
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	must.NoError(t, err)
	must.Len(t, 1, pool.Subjects())
}

func TestConfig_LoadKeyPair_None(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		KeyLoader: &config.KeyLoader{},
	}
	cert, err := conf.LoadKeyPair()
	must.NoError(t, err)
	must.Nil(t, cert, must.Sprint("expected no cert"))
}

func TestConfig_LoadKeyPair_Valid(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		CertFile:  fooclientcert,
		KeyFile:   fooclientkey,
		KeyLoader: &config.KeyLoader{},
	}
	cert, err := conf.LoadKeyPair()
	must.NoError(t, err)
	must.NotNil(t, cert)
}

func TestConfig_OutgoingTLS_MissingCA(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyOutgoing: true,
	}
	tls, err := conf.OutgoingTLSConfig()
	must.ErrorContains(t, err, "no CA certificate provided")
	must.Nil(t, tls, must.Sprint("expected no config"))
}

func TestConfig_OutgoingTLS_OnlyCA(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		CAFile: cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	must.NoError(t, err)
	must.Nil(t, tls, must.Sprint("expected no config"))
}

func TestConfig_OutgoingTLS_VerifyOutgoing(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	must.NoError(t, err)
	must.NotNil(t, tls)
	must.Len(t, 1, tls.RootCAs.Subjects(), must.Sprint("expected root cert"))
	must.True(t, tls.InsecureSkipVerify, must.Sprint("should skip built-in verification"))
}

func TestConfig_OutgoingTLS_VerifyHostname(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyServerHostname: true,
		CAFile:               cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	must.NoError(t, err)
	must.NotNil(t, tls)
	must.Len(t, 1, tls.RootCAs.Subjects(), must.Sprint("expected root cert"))
	must.False(t, tls.InsecureSkipVerify,
		must.Sprint("should not skip built-in verification"))
}

func TestConfig_OutgoingTLS_WithKeyPair(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         cacert,
		CertFile:       fooclientcert,
		KeyFile:        fooclientkey,
		KeyLoader:      &config.KeyLoader{},
	}
	tlsConf, err := conf.OutgoingTLSConfig()
	must.NoError(t, err)
	must.NotNil(t, tlsConf)
	must.Len(t, 1, tlsConf.RootCAs.Subjects())
	must.True(t, tlsConf.InsecureSkipVerify)

	clientHelloInfo := &tls.ClientHelloInfo{}
	cert, err := tlsConf.GetCertificate(clientHelloInfo)
	must.NoError(t, err)
	must.NotNil(t, cert)
}

func TestConfig_OutgoingTLS_TLSCipherSuites(t *testing.T) {
	ci.Parallel(t)

	{
		defaultCiphers := []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		}
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         cacert,
			CipherSuites:   defaultCiphers,
		}
		tlsConfig, err := conf.OutgoingTLSConfig()
		must.NoError(t, err)
		must.Eq(t, defaultCiphers, tlsConfig.CipherSuites)
	}
	{
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         cacert,
			CipherSuites:   []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
		}
		tlsConfig, err := conf.OutgoingTLSConfig()
		must.NoError(t, err)
		must.Eq(t, []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
			tlsConfig.CipherSuites)
	}
}

func startTLSServer(config *Config) (net.Conn, chan error) {
	errc := make(chan error, 1)

	tlsConfigServer, err := config.IncomingTLSConfig()
	if err != nil {
		errc <- err
		return nil, errc
	}

	client, server := net.Pipe()

	// Use yamux to buffer the reads, otherwise it's easy to deadlock
	muxConf := yamux.DefaultConfig()
	serverSession, _ := yamux.Server(server, muxConf)
	clientSession, _ := yamux.Client(client, muxConf)
	clientConn, _ := clientSession.Open()
	serverConn, _ := serverSession.Accept()

	go func() {
		tlsServer := tls.Server(serverConn, tlsConfigServer)
		if err := tlsServer.Handshake(); err != nil {
			errc <- err
		}
		close(errc)
		// Because net.Pipe() is unbuffered, if both sides
		// Close() simultaneously, we will deadlock as they
		// both send an alert and then block. So we make the
		// server read any data from the client until error or
		// EOF, which will allow the client to Close(), and
		// *then* we Close() the server.
		io.Copy(io.Discard, tlsServer)
		tlsServer.Close()
	}()
	return clientConn, errc
}

// TODO sign the certificates for "server.regionFoo.nomad
func TestConfig_outgoingWrapper_OK(t *testing.T) {
	ci.Parallel(t)

	config := &Config{
		CAFile:               cacert,
		CertFile:             fooservercert,
		KeyFile:              fooserverkey,
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		KeyLoader:            &config.KeyLoader{},
	}

	client, errc := startTLSServer(config)
	must.NotNil(t, client, must.Sprintf("%v", drainErrCh(t, errc)))

	wrap, err := config.OutgoingTLSWrapper()
	must.NoError(t, err)

	tlsClient, err := wrap("regionFoo", client)
	must.NoError(t, err)

	defer tlsClient.Close()
	err = tlsClient.(*tls.Conn).Handshake()
	must.NoError(t, err)
	must.NoError(t, drainErrCh(t, errc))
}

func TestConfig_outgoingWrapper_BadCert(t *testing.T) {
	ci.Parallel(t)
	config := &Config{
		CAFile:               cacert,
		CertFile:             fooclientcert,
		KeyFile:              fooclientkey,
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		KeyLoader:            &config.KeyLoader{},
	}

	client, errc := startTLSServer(config)
	if client == nil {
		err := drainErrCh(t, errc)
		t.Fatalf("startTLSServer err: %v", err)
	}

	wrap, err := config.OutgoingTLSWrapper()
	must.NoError(t, err, must.Sprint("OutgoingTLSWrapper err"))

	_, err = wrap("regionFoo", client)
	must.ErrorContains(t, err, "failed to verify certificate: x509")
	must.ErrorContains(t, drainErrCh(t, errc), "remote error: tls: bad certificate")
}

func TestConfig_wrapTLS_OK(t *testing.T) {
	ci.Parallel(t)

	config := &Config{
		CAFile:         cacert,
		CertFile:       fooclientcert,
		KeyFile:        fooclientkey,
		VerifyOutgoing: true,
		KeyLoader:      &config.KeyLoader{},
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	clientConfig, err := config.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("OutgoingTLSConfig err: %v", err)
	}

	tlsClient, err := WrapTLSClient(client, clientConfig)
	if err != nil {
		t.Fatalf("wrapTLS err: %v", err)
	} else {
		tlsClient.Close()
	}
	err = <-errc
	if err != nil {
		t.Fatalf("server: %v", err)
	}
}

func TestConfig_wrapTLS_BadCert(t *testing.T) {
	ci.Parallel(t)

	serverConfig := &Config{
		CAFile:    cacert,
		CertFile:  badcert,
		KeyFile:   badkey,
		KeyLoader: &config.KeyLoader{},
	}

	client, errc := startTLSServer(serverConfig)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	clientConfig := &Config{
		CAFile:         cacert,
		VerifyOutgoing: true,
	}

	clientTLSConfig, err := clientConfig.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("OutgoingTLSConfig err: %v", err)
	}

	tlsClient, err := WrapTLSClient(client, clientTLSConfig)
	if err == nil {
		t.Fatalf("wrapTLS no err")
	}
	if tlsClient != nil {
		t.Fatalf("returned a client")
	}

	// The server receives a TLS alert because the client aborted the
	// handshake after VerifyConnection rejected the bad certificate.
	// Drain the channel so the goroutine can exit.
	<-errc
}

func TestConfig_IncomingTLS(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyIncoming: true,
		CAFile:         cacert,
		CertFile:       fooclientcert,
		KeyFile:        fooclientkey,
		KeyLoader:      &config.KeyLoader{},
	}
	tlsC, err := conf.IncomingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tlsC == nil {
		t.Fatalf("expected config")
	}
	if len(tlsC.ClientCAs.Subjects()) != 1 {
		t.Fatalf("expect client cert")
	}
	if tlsC.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("should not skip verification")
	}

	clientHelloInfo := &tls.ClientHelloInfo{}
	cert, err := tlsC.GetCertificate(clientHelloInfo)
	must.NoError(t, err)
	must.NotNil(t, cert)
}

func TestConfig_IncomingTLS_MissingCA(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyIncoming: true,
		CertFile:       fooclientcert,
		KeyFile:        fooclientkey,
		KeyLoader:      &config.KeyLoader{},
	}
	_, err := conf.IncomingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestConfig_IncomingTLS_MissingKey(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{
		VerifyIncoming: true,
		CAFile:         cacert,
	}
	_, err := conf.IncomingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestConfig_IncomingTLS_NoVerify(t *testing.T) {
	ci.Parallel(t)

	conf := &Config{}
	tlsC, err := conf.IncomingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tlsC == nil {
		t.Fatalf("expected config")
	}
	if len(tlsC.ClientCAs.Subjects()) != 0 {
		t.Fatalf("do not expect client cert")
	}
	if tlsC.ClientAuth != tls.NoClientCert {
		t.Fatalf("should skip verification")
	}
	if len(tlsC.Certificates) != 0 {
		t.Fatalf("unexpected client cert")
	}
}

func TestConfig_IncomingTLS_TLSCipherSuites(t *testing.T) {
	ci.Parallel(t)

	{
		defaultCiphers := []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		}
		conf := &Config{
			CipherSuites: defaultCiphers,
		}
		tlsConfig, err := conf.IncomingTLSConfig()
		must.NoError(t, err)
		must.Eq(t, defaultCiphers, tlsConfig.CipherSuites)
	}
	{
		conf := &Config{
			CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
		}
		tlsConfig, err := conf.IncomingTLSConfig()
		must.NoError(t, err)
		must.Eq(t, []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
			tlsConfig.CipherSuites)
	}
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Valid(t *testing.T) {
	ci.Parallel(t)

	tlsConfig := &config.TLSConfig{
		CertFile:  fooclientcert,
		KeyFile:   fooclientkey,
		KeyLoader: &config.KeyLoader{},
		TLSCipherSuites: strings.Join([]string{
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
			"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
			"TLS_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_128_CBC_SHA",
			"TLS_RSA_WITH_AES_256_CBC_SHA",
		}, ","),
	}

	expectedCiphers := []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	parsedCiphers, err := ParseCiphers(tlsConfig)
	must.NoError(t, err)
	must.Eq(t, expectedCiphers, parsedCiphers)
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Default(t *testing.T) {
	ci.Parallel(t)

	expectedCiphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}

	empty := &config.TLSConfig{
		CertFile:  fooclientcert,
		KeyFile:   fooclientkey,
		KeyLoader: &config.KeyLoader{},
	}
	parsedCiphers, err := ParseCiphers(empty)
	must.NoError(t, err)
	must.Eq(t, expectedCiphers, parsedCiphers)
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Invalid(t *testing.T) {
	ci.Parallel(t)

	invalidCiphers := []string{
		"TLS_RSA_RSA_WITH_RC4_128_SHA",
		"INVALID_CIPHER",
	}

	for _, cipher := range invalidCiphers {
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: cipher,
			CertFile:        fooclientcert,
			KeyFile:         fooclientkey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		must.EqError(t, err, fmt.Sprintf("unsupported TLS cipher %q", cipher))
		must.Len(t, 0, parsedCiphers)
	}
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_SupportedSignature(t *testing.T) {
	ci.Parallel(t)

	// Supported signature
	{
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			CertFile:        fooclientcert,
			KeyFile:         fooclientkey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		must.NoError(t, err)
		must.Len(t, 1, parsedCiphers)
	}

	// Unsupported signature
	{
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			CertFile:        fooclientcert,
			KeyFile:         fooclientkey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		must.ErrorContains(t, err,
			"Specified cipher suites don't support the certificate signature algorithm")
		must.Len(t, 0, parsedCiphers)
	}
}

func TestConfig_ParseMinVersion_Valid(t *testing.T) {
	ci.Parallel(t)

	validVersions := []string{"tls10",
		"tls11",
		"tls12",
	}

	expected := map[string]uint16{
		"tls10": tls.VersionTLS10,
		"tls11": tls.VersionTLS11,
		"tls12": tls.VersionTLS12,
	}

	for _, version := range validVersions {
		parsedVersion, err := ParseMinVersion(version)
		must.NoError(t, err)
		must.Eq(t, expected[version], parsedVersion)
	}
}

func TestConfig_ParseMinVersion_Invalid(t *testing.T) {
	ci.Parallel(t)

	invalidVersions := []string{"ssl3", "tls14", "tls15"}

	for _, version := range invalidVersions {
		parsedVersion, err := ParseMinVersion(version)
		must.EqError(t, err, fmt.Sprintf("unsupported TLS version %q", version))
		must.Eq(t, uint16(0), parsedVersion)
	}
}

func TestConfig_NewTLSConfiguration(t *testing.T) {
	ci.Parallel(t)

	conf := &config.TLSConfig{
		TLSCipherSuites: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		CertFile:        fooclientcert,
		KeyFile:         fooclientkey,
		KeyLoader:       &config.KeyLoader{},
	}

	tlsConf, err := NewTLSConfiguration(conf, true, true)
	must.NoError(t, err)
	must.True(t, tlsConf.VerifyIncoming)
	must.True(t, tlsConf.VerifyOutgoing)

	expectedCiphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
	must.Eq(t, expectedCiphers, tlsConf.CipherSuites)
}

func TestConfig_ShouldReloadRPCConnections(t *testing.T) {
	ci.Parallel(t)

	type shouldReloadTestInput struct {
		old          *config.TLSConfig
		new          *config.TLSConfig
		expectReload bool
		errorStr     string
	}

	testInput := []*shouldReloadTestInput{
		{
			old: &config.TLSConfig{
				CAFile:   cacert,
				CertFile: badcert,
				KeyFile:  badkey,
			},
			new: &config.TLSConfig{
				CAFile:   cacert,
				CertFile: badcert,
				KeyFile:  badkey,
			},
			expectReload: false,
			errorStr:     "Same TLS Configuration should not reload",
		},
		{
			old: &config.TLSConfig{
				CAFile:   cacert,
				CertFile: badcert,
				KeyFile:  badkey,
			},
			new: &config.TLSConfig{
				CAFile:   cacert,
				CertFile: fooclientcert,
				KeyFile:  fooclientkey,
			},
			expectReload: true,
			errorStr:     "Different TLS Configuration should reload",
		},
		{
			old: &config.TLSConfig{
				CAFile:    cacert,
				CertFile:  badcert,
				KeyFile:   badkey,
				EnableRPC: true,
			},
			new: &config.TLSConfig{
				CAFile:    cacert,
				CertFile:  badcert,
				KeyFile:   badkey,
				EnableRPC: false,
			},
			expectReload: true,
			errorStr:     "Downgrading RPC connections should force reload",
		},
		{
			old: nil,
			new: &config.TLSConfig{
				CAFile:    cacert,
				CertFile:  badcert,
				KeyFile:   badkey,
				EnableRPC: true,
			},
			expectReload: true,
			errorStr:     "Upgrading RPC connections should force reload",
		},
		{
			old: &config.TLSConfig{
				CAFile:    cacert,
				CertFile:  badcert,
				KeyFile:   badkey,
				EnableRPC: true,
			},
			new:          nil,
			expectReload: true,
			errorStr:     "Downgrading RPC connections should force reload",
		},
	}

	for _, testCase := range testInput {
		shouldReload, err := ShouldReloadRPCConnections(testCase.old, testCase.new)
		must.NoError(t, err)
		must.Eq(t, testCase.expectReload, shouldReload, must.Sprint(testCase.errorStr))
	}
}

func drainErrCh(t *testing.T, errc chan error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// See README.md for documentation
	cacert  = "./testdata/ca.pem"
	foocert = "./testdata/nomad-foo.pem"
	fookey  = "./testdata/nomad-foo-key.pem"
	badcert = "./testdata/nomad-bad.pem"
	badkey  = "./testdata/nomad-bad-key.pem"
)

func TestConfig_AppendCA_None(t *testing.T) {
	require := require.New(t)

	conf := &Config{}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)

	require.Nil(err)
}

func TestConfig_AppendCA_Valid(t *testing.T) {
	require := require.New(t)

	conf := &Config{
		CAFile: cacert,
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)

	require.Nil(err)
}

func TestConfig_AppendCA_Valid_MultipleCerts(t *testing.T) {
	require := require.New(t)

	tmpCAFile, err := ioutil.TempFile("/tmp", "test_ca_file")
	require.Nil(err)
	defer os.Remove(tmpCAFile.Name())

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
-----END CERTIFICATE-----`

	_, err = tmpCAFile.Write([]byte(certs))
	require.Nil(err)

	conf := &Config{
		CAFile: tmpCAFile.Name(),
	}
	pool := x509.NewCertPool()
	err = conf.AppendCA(pool)

	require.Nil(err)
}

func TestConfig_AppendCA_InValid_MultipleCerts(t *testing.T) {
	require := require.New(t)

	tmpCAFile, err := ioutil.TempFile("/tmp", "test_ca_file")
	require.Nil(err)
	defer os.Remove(tmpCAFile.Name())

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

	_, err = tmpCAFile.Write([]byte(certs))
	require.Nil(err)

	conf := &Config{
		CAFile: tmpCAFile.Name(),
	}
	pool := x509.NewCertPool()
	err = conf.AppendCA(pool)

	require.NotNil(err)
}

func TestConfig_AppendCA_Invalid(t *testing.T) {
	require := require.New(t)
	{
		conf := &Config{
			CAFile: "invalidFile",
		}
		pool := x509.NewCertPool()
		err := conf.AppendCA(pool)
		require.NotNil(err)
		require.Contains(err.Error(), "Failed to read CA file")
		require.Equal(len(pool.Subjects()), 0)
	}

	{
		tmpFile, err := ioutil.TempFile("/tmp", "test_ca_file")
		require.Nil(err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write([]byte("Invalid CA Content!"))
		require.Nil(err)

		conf := &Config{
			CAFile: tmpFile.Name(),
		}
		pool := x509.NewCertPool()
		err = conf.AppendCA(pool)
		require.NotNil(err)
		require.Contains(err.Error(), "Failed to decode CA file from pem format")
		require.Equal(len(pool.Subjects()), 0)
	}
}

func TestConfig_CACertificate_Valid(t *testing.T) {
	conf := &Config{
		CAFile: cacert,
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) == 0 {
		t.Fatalf("expected cert")
	}
}

func TestConfig_LoadKeyPair_None(t *testing.T) {
	conf := &Config{
		KeyLoader: &config.KeyLoader{},
	}
	cert, err := conf.LoadKeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert != nil {
		t.Fatalf("bad: %v", cert)
	}
}

func TestConfig_LoadKeyPair_Valid(t *testing.T) {
	conf := &Config{
		CertFile:  foocert,
		KeyFile:   fookey,
		KeyLoader: &config.KeyLoader{},
	}
	cert, err := conf.LoadKeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert == nil {
		t.Fatalf("expected cert")
	}
}

func TestConfig_OutgoingTLS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
	}
	tls, err := conf.OutgoingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
	if tls != nil {
		t.Fatalf("bad: %v", tls)
	}
}

func TestConfig_OutgoingTLS_OnlyCA(t *testing.T) {
	conf := &Config{
		CAFile: cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls != nil {
		t.Fatalf("expected no config")
	}
}

func TestConfig_OutgoingTLS_VerifyOutgoing(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls == nil {
		t.Fatalf("expected config")
	}
	if len(tls.RootCAs.Subjects()) != 1 {
		t.Fatalf("expect root cert")
	}
	if !tls.InsecureSkipVerify {
		t.Fatalf("should skip built-in verification")
	}
}

func TestConfig_OutgoingTLS_VerifyHostname(t *testing.T) {
	conf := &Config{
		VerifyServerHostname: true,
		CAFile:               cacert,
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls == nil {
		t.Fatalf("expected config")
	}
	if len(tls.RootCAs.Subjects()) != 1 {
		t.Fatalf("expect root cert")
	}
	if tls.InsecureSkipVerify {
		t.Fatalf("should not skip built-in verification")
	}
}

func TestConfig_OutgoingTLS_WithKeyPair(t *testing.T) {
	assert := assert.New(t)

	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         cacert,
		CertFile:       foocert,
		KeyFile:        fookey,
		KeyLoader:      &config.KeyLoader{},
	}
	tlsConf, err := conf.OutgoingTLSConfig()
	assert.Nil(err)
	assert.NotNil(tlsConf)
	assert.Equal(len(tlsConf.RootCAs.Subjects()), 1)
	assert.True(tlsConf.InsecureSkipVerify)

	clientHelloInfo := &tls.ClientHelloInfo{}
	cert, err := tlsConf.GetCertificate(clientHelloInfo)
	assert.Nil(err)
	assert.NotNil(cert)
}

func TestConfig_OutgoingTLS_PreferServerCipherSuites(t *testing.T) {
	require := require.New(t)

	{
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         cacert,
		}
		tlsConfig, err := conf.OutgoingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.PreferServerCipherSuites, false)
	}
	{
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         cacert,
			PreferServerCipherSuites: true,
		}
		tlsConfig, err := conf.OutgoingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.PreferServerCipherSuites, true)
	}
}

func TestConfig_OutgoingTLS_TLSCipherSuites(t *testing.T) {
	require := require.New(t)

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
		require.Nil(err)
		require.Equal(tlsConfig.CipherSuites, defaultCiphers)
	}
	{
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         cacert,
			CipherSuites:   []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
		}
		tlsConfig, err := conf.OutgoingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.CipherSuites, []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305})
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
		io.Copy(ioutil.Discard, tlsServer)
		tlsServer.Close()
	}()
	return clientConn, errc
}

// TODO sign the certificates for "server.regionFoo.nomad
func TestConfig_outgoingWrapper_OK(t *testing.T) {
	config := &Config{
		CAFile:               cacert,
		CertFile:             foocert,
		KeyFile:              fookey,
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		KeyLoader:            &config.KeyLoader{},
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	wrap, err := config.OutgoingTLSWrapper()
	if err != nil {
		t.Fatalf("OutgoingTLSWrapper err: %v", err)
	}

	tlsClient, err := wrap("regionFoo", client)
	if err != nil {
		t.Fatalf("wrapTLS err: %v", err)
	}
	defer tlsClient.Close()
	if err := tlsClient.(*tls.Conn).Handshake(); err != nil {
		t.Fatalf("write err: %v", err)
	}

	err = <-errc
	if err != nil {
		t.Fatalf("server: %v", err)
	}
}

func TestConfig_outgoingWrapper_BadCert(t *testing.T) {
	// TODO this test is currently hanging, need to investigate more.
	t.SkipNow()
	config := &Config{
		CAFile:               cacert,
		CertFile:             foocert,
		KeyFile:              fookey,
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	wrap, err := config.OutgoingTLSWrapper()
	if err != nil {
		t.Fatalf("OutgoingTLSWrapper err: %v", err)
	}

	tlsClient, err := wrap("regionFoo", client)
	if err != nil {
		t.Fatalf("wrapTLS err: %v", err)
	}
	defer tlsClient.Close()
	err = tlsClient.(*tls.Conn).Handshake()

	if _, ok := err.(x509.HostnameError); !ok {
		t.Fatalf("should get hostname err: %v", err)
	}

	<-errc
}

func TestConfig_wrapTLS_OK(t *testing.T) {
	config := &Config{
		CAFile:         cacert,
		CertFile:       foocert,
		KeyFile:        fookey,
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

	err = <-errc
	if err != nil {
		t.Fatalf("server: %v", err)
	}
}

func TestConfig_IncomingTLS(t *testing.T) {
	assert := assert.New(t)

	conf := &Config{
		VerifyIncoming: true,
		CAFile:         cacert,
		CertFile:       foocert,
		KeyFile:        fookey,
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
	assert.Nil(err)
	assert.NotNil(cert)
}

func TestConfig_IncomingTLS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CertFile:       foocert,
		KeyFile:        fookey,
		KeyLoader:      &config.KeyLoader{},
	}
	_, err := conf.IncomingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestConfig_IncomingTLS_MissingKey(t *testing.T) {
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

func TestConfig_IncomingTLS_PreferServerCipherSuites(t *testing.T) {
	require := require.New(t)

	{
		conf := &Config{}
		tlsConfig, err := conf.IncomingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.PreferServerCipherSuites, false)
	}
	{
		conf := &Config{
			PreferServerCipherSuites: true,
		}
		tlsConfig, err := conf.IncomingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.PreferServerCipherSuites, true)
	}
}

func TestConfig_IncomingTLS_TLSCipherSuites(t *testing.T) {
	require := require.New(t)

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
		require.Nil(err)
		require.Equal(tlsConfig.CipherSuites, defaultCiphers)
	}
	{
		conf := &Config{
			CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
		}
		tlsConfig, err := conf.IncomingTLSConfig()
		require.Nil(err)
		require.Equal(tlsConfig.CipherSuites, []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305})
	}
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Valid(t *testing.T) {
	require := require.New(t)

	tlsConfig := &config.TLSConfig{
		CertFile:  foocert,
		KeyFile:   fookey,
		KeyLoader: &config.KeyLoader{},
		TLSCipherSuites: strings.Join([]string{
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
			"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
			"TLS_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_128_CBC_SHA256",
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
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	parsedCiphers, err := ParseCiphers(tlsConfig)
	require.Nil(err)
	require.Equal(parsedCiphers, expectedCiphers)
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Default(t *testing.T) {
	require := require.New(t)

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
		CertFile:  foocert,
		KeyFile:   fookey,
		KeyLoader: &config.KeyLoader{},
	}
	parsedCiphers, err := ParseCiphers(empty)
	require.Nil(err)
	require.Equal(parsedCiphers, expectedCiphers)
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_Invalid(t *testing.T) {
	require := require.New(t)

	invalidCiphers := []string{
		"TLS_RSA_RSA_WITH_RC4_128_SHA",
		"INVALID_CIPHER",
	}

	for _, cipher := range invalidCiphers {
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: cipher,
			CertFile:        foocert,
			KeyFile:         fookey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		require.NotNil(err)
		require.Equal(fmt.Sprintf("unsupported TLS cipher %q", cipher), err.Error())
		require.Equal(0, len(parsedCiphers))
	}
}

// This test relies on the fact that the specified certificate has an ECDSA
// signature algorithm
func TestConfig_ParseCiphers_SupportedSignature(t *testing.T) {
	require := require.New(t)

	// Supported signature
	{
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			CertFile:        foocert,
			KeyFile:         fookey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		require.Nil(err)
		require.Equal(1, len(parsedCiphers))
	}

	// Unsupported signature
	{
		tlsConfig := &config.TLSConfig{
			TLSCipherSuites: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			CertFile:        foocert,
			KeyFile:         fookey,
			KeyLoader:       &config.KeyLoader{},
		}
		parsedCiphers, err := ParseCiphers(tlsConfig)
		require.NotNil(err)
		require.Equal(0, len(parsedCiphers))
	}
}

func TestConfig_ParseMinVersion_Valid(t *testing.T) {
	require := require.New(t)

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
		require.Nil(err)
		require.Equal(expected[version], parsedVersion)
	}
}

func TestConfig_ParseMinVersion_Invalid(t *testing.T) {
	require := require.New(t)

	invalidVersions := []string{"tls13",
		"tls15",
	}

	for _, version := range invalidVersions {
		parsedVersion, err := ParseMinVersion(version)
		require.NotNil(err)
		require.Equal(fmt.Sprintf("unsupported TLS version %q", version), err.Error())
		require.Equal(uint16(0), parsedVersion)
	}
}

func TestConfig_NewTLSConfiguration(t *testing.T) {
	require := require.New(t)

	conf := &config.TLSConfig{
		TLSCipherSuites: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		CertFile:        foocert,
		KeyFile:         fookey,
		KeyLoader:       &config.KeyLoader{},
	}

	tlsConf, err := NewTLSConfiguration(conf, true, true)
	require.Nil(err)
	require.True(tlsConf.VerifyIncoming)
	require.True(tlsConf.VerifyOutgoing)

	expectedCiphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
	require.Equal(tlsConf.CipherSuites, expectedCiphers)
}

func TestConfig_ShouldReloadRPCConnections(t *testing.T) {
	require := require.New(t)

	type shouldReloadTestInput struct {
		old          *config.TLSConfig
		new          *config.TLSConfig
		shouldReload bool
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
			shouldReload: false,
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
				CertFile: foocert,
				KeyFile:  fookey,
			},
			shouldReload: true,
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
			shouldReload: true,
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
			shouldReload: true,
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
			shouldReload: true,
			errorStr:     "Downgrading RPC connections should force reload",
		},
	}

	for _, testCase := range testInput {
		shouldReload, err := ShouldReloadRPCConnections(testCase.old, testCase.new)
		require.NoError(err)
		require.Equal(shouldReload, testCase.shouldReload, testCase.errorStr)
	}
}

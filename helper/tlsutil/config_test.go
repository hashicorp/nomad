package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
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
	conf := &Config{}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) != 0 {
		t.Fatalf("bad: %v", pool.Subjects())
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

func TestConfig_ParseCiphers_Valid(t *testing.T) {
	require := require.New(t)

	validCiphers := strings.Join([]string{
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
	}, ",")

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

	parsedCiphers, err := ParseCiphers(validCiphers)
	require.Nil(err)
	require.Equal(parsedCiphers, expectedCiphers)
}

func TestConfig_ParseCiphers_Default(t *testing.T) {
	require := require.New(t)

	expectedCiphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}

	parsedCiphers, err := ParseCiphers("")
	require.Nil(err)
	require.Equal(parsedCiphers, expectedCiphers)
}

func TestConfig_ParseCiphers_Invalid(t *testing.T) {
	require := require.New(t)

	invalidCiphers := []string{"TLS_RSA_WITH_3DES_EDE_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA",
		"TLS_RSA_WITH_RC4_128_SHA",
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA",
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA",
	}

	for _, cipher := range invalidCiphers {
		parsedCiphers, err := ParseCiphers(cipher)
		require.NotNil(err)
		require.Equal(fmt.Sprintf("unsupported TLS cipher %q", cipher), err.Error())
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

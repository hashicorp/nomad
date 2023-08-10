// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tlsutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestSerialNumber(t *testing.T) {
	n1, err := GenerateSerialNumber()
	require.Nil(t, err)

	n2, err := GenerateSerialNumber()
	require.Nil(t, err)
	require.NotEqual(t, n1, n2)

	n3, err := GenerateSerialNumber()
	require.Nil(t, err)
	require.NotEqual(t, n1, n3)
	require.NotEqual(t, n2, n3)

}

func TestGeneratePrivateKey(t *testing.T) {
	ci.Parallel(t)
	_, p, err := GeneratePrivateKey()
	require.Nil(t, err)
	require.NotEmpty(t, p)
	require.Contains(t, p, "BEGIN EC PRIVATE KEY")
	require.Contains(t, p, "END EC PRIVATE KEY")

	block, _ := pem.Decode([]byte(p))
	pk, err := x509.ParseECPrivateKey(block.Bytes)

	require.Nil(t, err)
	require.NotNil(t, pk)
	require.Equal(t, 256, pk.Params().BitSize)
}

type TestSigner struct {
	public interface{}
}

func (s *TestSigner) Public() crypto.PublicKey {
	return s.public
}

func (s *TestSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return []byte{}, nil
}

func TestGenerateCA(t *testing.T) {
	ci.Parallel(t)

	t.Run("no signer", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{Signer: &TestSigner{}})
		require.Error(t, err)
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("wrong key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{Signer: &TestSigner{public: &rsa.PublicKey{}}})
		require.Error(t, err)
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("valid key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{})
		require.Nil(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.Nil(t, err)
		require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Nomad Agent CA"))
		require.Equal(t, true, cert.IsCA)
		require.Equal(t, true, cert.BasicConstraintsValid)

		require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 1825), time.Minute)

		require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
	})

	t.Run("RSA key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{})
		require.NoError(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Nomad Agent CA"))
		require.Equal(t, true, cert.IsCA)
		require.Equal(t, true, cert.BasicConstraintsValid)

		require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 1825), time.Minute)

		require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
	})

	t.Run("Custom CA", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days:                6,
			PermittedDNSDomains: []string{"domain1.com"},
			Country:             "ZZ",
			PostalCode:          "0000",
			Province:            "CustProvince",
			Locality:            "CustLocality",
			StreetAddress:       "CustStreet",
			Organization:        "CustOrg",
			OrganizationalUnit:  "CustUnit",
			Name:                "Custom CA",
		})
		require.NoError(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Custom CA"))
		require.True(t, strings.Contains(cert.PermittedDNSDomains[0], "domain1.com"))
		require.True(t, strings.Contains(cert.Subject.Country[0], "ZZ"))
		require.True(t, strings.Contains(cert.Subject.PostalCode[0], "0000"))
		require.True(t, strings.Contains(cert.Subject.Province[0], "CustProvince"))
		require.True(t, strings.Contains(cert.Subject.Locality[0], "CustLocality"))
		require.True(t, strings.Contains(cert.Subject.StreetAddress[0], "CustStreet"))
		require.True(t, strings.Contains(cert.Subject.Organization[0], "CustOrg"))
		require.True(t, strings.Contains(cert.Subject.OrganizationalUnit[0], "CustUnit"))
		require.Equal(t, true, cert.IsCA)
		require.Equal(t, true, cert.BasicConstraintsValid)

		require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 6), time.Minute)

		require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
	})

	t.Run("Custom CA Custom Date", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days: 365,
		})
		require.NoError(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 365), time.Minute)
	})

	t.Run("Custom CA No CN", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days:                6,
			PermittedDNSDomains: []string{"domain1.com"},
			Locality:            "CustLocality",
		})
		require.ErrorContains(t, err, "common name value not provided")
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("Custom CA No Country", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days:                6,
			PermittedDNSDomains: []string{"domain1.com"},
			Name:                "Custom CA",
			Locality:            "CustLocality",
		})
		require.ErrorContains(t, err, "country value not provided")
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("Custom CA No Organization", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days:                6,
			PermittedDNSDomains: []string{"domain1.com"},
			Name:                "Custom CA",
			Country:             "ZZ",
			Locality:            "CustLocality",
		})
		require.ErrorContains(t, err, "organization value not provided")
		// require.NoError(t, err)
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("Custom CA No Organizational Unit", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{
			Days:                6,
			PermittedDNSDomains: []string{"domain1.com"},
			Name:                "Custom CA",
			Country:             "ZZ",
			Locality:            "CustLocality",
			Organization:        "CustOrg",
		})
		require.ErrorContains(t, err, "organizational unit value not provided")
		require.Empty(t, ca)
		require.Empty(t, pk)
	})
}

func TestGenerateCert(t *testing.T) {
	ci.Parallel(t)

	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	ca, _, err := GenerateCA(CAOpts{
		Name:               "Custom CA",
		Country:            "ZZ",
		Organization:       "CustOrg",
		OrganizationalUnit: "CustOrgUnit",
		Signer:             signer},
	)
	require.Nil(t, err)

	DNSNames := []string{"server.dc1.nomad"}
	IPAddresses := []net.IP{net.ParseIP("123.234.243.213")}
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	name := "Cert Name"
	certificate, pk, err := GenerateCert(CertOpts{
		Signer: signer, CA: ca, Name: name, Days: 365,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	require.Nil(t, err)
	require.NotEmpty(t, certificate)
	require.NotEmpty(t, pk)

	cert, err := parseCert(certificate)
	require.Nil(t, err)
	require.Equal(t, name, cert.Subject.CommonName)
	require.Equal(t, true, cert.BasicConstraintsValid)
	signee, err := ParseSigner(pk)
	require.Nil(t, err)
	certID, err := keyID(signee.Public())
	require.Nil(t, err)
	require.Equal(t, certID, cert.SubjectKeyId)
	caID, err := keyID(signer.Public())
	require.Nil(t, err)
	require.Equal(t, caID, cert.AuthorityKeyId)
	require.Contains(t, cert.Issuer.CommonName, "Custom CA")
	require.Equal(t, false, cert.IsCA)

	require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
	require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 365), time.Minute)

	require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
	require.Equal(t, extKeyUsage, cert.ExtKeyUsage)

	// https://github.com/golang/go/blob/10538a8f9e2e718a47633ac5a6e90415a2c3f5f1/src/crypto/x509/verify.go#L414
	require.Equal(t, DNSNames, cert.DNSNames)
	require.True(t, IPAddresses[0].Equal(cert.IPAddresses[0]))
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tlsutil

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

// GenerateSerialNumber returns random bigint generated with crypto/rand
func GenerateSerialNumber() (*big.Int, error) {
	l := new(big.Int).Lsh(big.NewInt(1), 128)
	s, err := rand.Int(rand.Reader, l)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GeneratePrivateKey generates a new ecdsa private key
func GeneratePrivateKey() (crypto.Signer, string, error) {
	curve := elliptic.P256()

	pk, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("error generating ECDSA private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return nil, "", fmt.Errorf("error marshaling ECDSA private key: %s", err)
	}

	pemBlock, err := pemEncodeKey(bs, "EC PRIVATE KEY")
	if err != nil {
		return nil, "", err
	}

	return pk, pemBlock, nil
}

func pemEncodeKey(key []byte, blockType string) (string, error) {
	var buf bytes.Buffer

	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: key}); err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}
	return buf.String(), nil
}

type CAOpts struct {
	Signer              crypto.Signer
	Serial              *big.Int
	Days                int
	PermittedDNSDomains []string
	Country             string
	PostalCode          string
	Province            string
	Locality            string
	StreetAddress       string
	Organization        string
	OrganizationalUnit  string
	Name                string
}

type CertOpts struct {
	Signer      crypto.Signer
	CA          string
	Serial      *big.Int
	Name        string
	Days        int
	DNSNames    []string
	IPAddresses []net.IP
	ExtKeyUsage []x509.ExtKeyUsage
}

// IsNotCustom checks whether any of CAOpts parameters have been populated with
// non-default values.
func (c *CAOpts) IsNotCustom() bool {
	return c.Country == "" &&
		c.PostalCode == "" &&
		c.Province == "" &&
		c.Locality == "" &&
		c.StreetAddress == "" &&
		c.Organization == "" &&
		c.OrganizationalUnit == "" &&
		c.Name == ""
}

// GenerateCA generates a new CA for agent TLS (not to be confused with Connect TLS)
func GenerateCA(opts CAOpts) (string, string, error) {
	var (
		id     []byte
		pk     string
		err    error
		signer = opts.Signer
		sn     = opts.Serial
	)
	if signer == nil {
		var err error
		signer, pk, err = GeneratePrivateKey()
		if err != nil {
			return "", "", err
		}
	}

	id, err = keyID(signer.Public())
	if err != nil {
		return "", "", err
	}

	if sn == nil {
		var err error
		sn, err = GenerateSerialNumber()
		if err != nil {
			return "", "", err
		}
	}

	if opts.IsNotCustom() {
		opts.Name = fmt.Sprintf("Nomad Agent CA %d", sn)
		if opts.Days == 0 {
			opts.Days = 1825
		}
		opts.Country = "US"
		opts.PostalCode = "94105"
		opts.Province = "CA"
		opts.Locality = "San Francisco"
		opts.StreetAddress = "101 Second Street"
		opts.Organization = "HashiCorp Inc."
		opts.OrganizationalUnit = "Nomad"
	} else {
		if opts.Name == "" {
			return "", "", errors.New("common name value not provided")
		} else {
			opts.Name = fmt.Sprintf("%s %d", opts.Name, sn)
		}
		if opts.Country == "" {
			return "", "", errors.New("country value not provided")
		}

		if opts.Organization == "" {
			return "", "", errors.New("organization value not provided")
		}

		if opts.OrganizationalUnit == "" {
			return "", "", errors.New("organizational unit value not provided")
		}
	}

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			Country:            []string{opts.Country},
			PostalCode:         []string{opts.PostalCode},
			Province:           []string{opts.Province},
			Locality:           []string{opts.Locality},
			StreetAddress:      []string{opts.StreetAddress},
			Organization:       []string{opts.Organization},
			OrganizationalUnit: []string{opts.OrganizationalUnit},
			CommonName:         opts.Name,
		},
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		NotAfter:              time.Now().AddDate(0, 0, opts.Days),
		NotBefore:             time.Now(),
		AuthorityKeyId:        id,
		SubjectKeyId:          id,
	}

	if len(opts.PermittedDNSDomains) > 0 {
		template.PermittedDNSDomainsCritical = true
		template.PermittedDNSDomains = opts.PermittedDNSDomains
	}
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, &template, signer.Public(), signer)
	if err != nil {
		return "", "", fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), pk, nil
}

// GenerateCert generates a new certificate for agent TLS (not to be confused with Connect TLS)
func GenerateCert(opts CertOpts) (string, string, error) {
	parent, err := parseCert(opts.CA)
	if err != nil {
		return "", "", err
	}

	signee, pk, err := GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}

	id, err := keyID(signee.Public())
	if err != nil {
		return "", "", err
	}

	sn := opts.Serial
	if sn == nil {
		var err error
		sn, err = GenerateSerialNumber()
		if err != nil {
			return "", "", err
		}
	}

	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: opts.Name},
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           opts.ExtKeyUsage,
		IsCA:                  false,
		NotAfter:              time.Now().AddDate(0, 0, opts.Days),
		NotBefore:             time.Now(),
		SubjectKeyId:          id,
		DNSNames:              opts.DNSNames,
		IPAddresses:           opts.IPAddresses,
	}

	bs, err := x509.CreateCertificate(rand.Reader, &template, parent, signee.Public(), opts.Signer)
	if err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), pk, nil
}

// KeyId returns a x509 KeyId from the given signing key.
func keyID(raw interface{}) ([]byte, error) {
	switch raw.(type) {
	case *ecdsa.PublicKey:
	case *rsa.PublicKey:
	default:
		return nil, fmt.Errorf("invalid key type: %T", raw)
	}

	// This is not standard; RFC allows any unique identifier as long as they
	// match in subject/authority chains but suggests specific hashing of DER
	// bytes of public key including DER tags.
	bs, err := x509.MarshalPKIXPublicKey(raw)
	if err != nil {
		return nil, err
	}

	// String formatted
	kID := sha256.Sum256(bs)
	return kID[:], nil
}

// ParseCert parses the x509 certificate from a PEM-encoded value.
func ParseCert(pemValue string) (*x509.Certificate, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("first PEM-block should be CERTIFICATE type")
	}

	return x509.ParseCertificate(block.Bytes)
}

func parseCert(pemValue string) (*x509.Certificate, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("first PEM-block should be CERTIFICATE type")
	}

	return x509.ParseCertificate(block.Bytes)
}

// ParseSigner parses a crypto.Signer from a PEM-encoded key. The private key
// is expected to be the first block in the PEM value.
func ParseSigner(pemValue string) (crypto.Signer, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)

	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)

	case "PRIVATE KEY":
		signer, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pk, ok := signer.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("private key is not a valid format")
		}

		return pk, nil

	default:
		return nil, fmt.Errorf("unknown PEM block type for signing key: %s", block.Type)
	}
}

func Verify(caString, certString, dns string) error {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(caString))
	if !ok {
		return fmt.Errorf("failed to parse root certificate")
	}

	cert, err := parseCert(certString)
	if err != nil {
		return fmt.Errorf("failed to parse certificate")
	}

	opts := x509.VerifyOptions{
		DNSName: fmt.Sprint(dns),
		Roots:   roots,
	}

	_, err = cert.Verify(opts)
	return err
}

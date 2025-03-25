// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"bytes"
	"crypto/rsa"
	// sha1 is used to derive an "x5t" jwt header from an x509 certificate,
	// per the OIDC JWS spec:
	// https://datatracker.ietf.org/doc/html/rfc7515#section-4.1.7
	// sha1 is not a security risk here, but it is less reliable than sha256
	// (for "x5t#S256" headers) in terms of possible value collisions,
	// so "x5t" must be set explicitly by the user in their auth method config
	// if their provider does not allow "x5t#S256" (the default).
	// None of this applies if the user sets the KeyID ("kid" header) manually.
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	cass "github.com/hashicorp/cap/oidc/clientassertion"
	"github.com/hashicorp/nomad/nomad/structs"
)

// BuildClientAssertionJWT makes a JWT to be included in an OIDC auth request.
// Reference: https://oauth.net/private-key-jwt/
//
// There are three input variations depending on OIDCClientAssertion.KeySource:
//   - "client_secret": uses the config's ClientSecret as an HMAC key to sign
//     the JWT. This is marginally more secure than a bare ClientSecret, as the
//     JWT is time-bound, and signed by the secret rather than sending the
//     secret itself over the network.
//   - "nomad": uses the RS256 nomadKey (Nomad's private key) to sign the JWT,
//     and the nomadKID as the JWT's "kid" header, which the OIDC provider uses
//     to find the public key at Nomad's JWKS endpoint (/.well-known/jwks.json)
//     to verify the JWT signature. This is arguably the most secure option,
//     because only Nomad has the private key.
//   - "private_key": uses an RSA private key provided by the user. They may
//     provide a KeyID to use as the JWT's "kid" header, or an x509 public
//     certificate to derive an x5t#S256 (or x5t) header, which the OIDC
//     provider uses to find the cert on their end to verify the JWT signature.
//     This is the most flexible option, allowing users to manage their own
//     keys however they like.
func BuildClientAssertionJWT(config *structs.ACLAuthMethodConfig, nomadKey *rsa.PrivateKey, nomadKID string) (*cass.JWT, error) {
	// should already be validated by caller, but just in case.
	if config == nil || config.OIDCClientAssertion == nil {
		return nil, errors.New("no auth method config or client assertion")
	}

	// this is all we use config for
	clientID := config.OIDCClientID
	// client assertion-specific info is in here
	as := config.OIDCClientAssertion

	// this should have also happened long before, but again, just in case.
	if err := as.Validate(); err != nil {
		return nil, err
	}

	opts := []cass.Option{
		cass.WithHeaders(as.ExtraHeaders),
	}

	switch as.KeySource {

	case structs.OIDCKeySourceClientSecret:
		algo := cass.HSAlgorithm(as.KeyAlgorithm)
		return cass.NewJWTWithHMAC(clientID, as.Audience, algo, as.ClientSecret, opts...)

	case structs.OIDCKeySourceNomad:
		opts = append(opts, cass.WithKeyID(nomadKID))
		return cass.NewJWTWithRSAKey(clientID, as.Audience, cass.RS256, nomadKey, opts...)

	case structs.OIDCKeySourcePrivateKey:
		algo := cass.RSAlgorithm(as.KeyAlgorithm)
		rsaKey, err := getCassPrivateKey(as.PrivateKey)
		if err != nil {
			return nil, err
		}

		if as.PrivateKey.KeyID != "" {
			// if the user provides a verbatim KeyID, set it as "kid" header
			opts = append(opts,
				cass.WithKeyID(as.PrivateKey.KeyID),
			)
		} else {
			// otherwise, derive it from the cert
			cert, err := getCassCert(as.PrivateKey)
			if err != nil {
				return nil, err
			}
			keyID, err := hashKeyID(cert, as.PrivateKey.KeyIDHeader)
			if err != nil {
				return nil, err
			}
			opts = append(opts, cass.WithHeaders(map[string]string{
				string(as.PrivateKey.KeyIDHeader): keyID,
			}))
		}
		return cass.NewJWTWithRSAKey(clientID, as.Audience, algo, rsaKey, opts...)

	default: // this shouldn't happen, but just in case
		return nil, fmt.Errorf("unknown OIDC KeySource %q", as.KeySource)
	}
}

// getCassPrivateKey parses the structs.OIDCClientAssertionKey PemKeyFile
// or PemKey, depending on which is set.
func getCassPrivateKey(k *structs.OIDCClientAssertionKey) (key *rsa.PrivateKey, err error) {
	var bts []byte
	var source string // for informative error messages

	// pem file on disk
	if k.PemKeyFile != "" {
		source = "PemKeyFile"
		bts, err = os.ReadFile(k.PemKeyFile)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", source, err)
		}
	}
	// or pem string
	if k.PemKey != "" {
		source = "PemKey"
		bts = []byte(k.PemKey)
	}

	// for easy copy-paste, users may leave off PEM header/footer
	bts = wrapBeginEnd(bts, beginPrivateKey, endPrivateKey)

	key, err = gojwt.ParseRSAPrivateKeyFromPEM(bts)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", source, err)
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("error validating %s: %w", source, err)
	}
	return key, nil
}

// getCassCert parses the structs.OIDCClientAssertionKey PemCertFile
// or PemCert, depending on which is set.
func getCassCert(k *structs.OIDCClientAssertionKey) (*x509.Certificate, error) {
	var bts []byte
	var err error
	var source string // for informative error messages

	// pem file on disk
	if k.PemCertFile != "" {
		source = "PemCertFile"
		bts, err = os.ReadFile(k.PemCertFile)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", source, err)
		}
	}
	// or pem string
	if k.PemCert != "" {
		source = "PemCert"
		bts = []byte(k.PemCert)
	}

	// for easy copy-paste, users may leave off PEM header/footer
	bts = wrapBeginEnd(bts, beginCertificate, endCertificate)

	block, _ := pem.Decode(bts)
	if block == nil {
		return nil, fmt.Errorf("failed to decode %s PEM block", source)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s bytes: %w", source, err)
	}
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return nil, errors.New("certificate has expired or is not yet valid")
	}
	return cert, nil
}

// hashKeyID derives a "certificate thumbprint" that the OIDC provider uses
// to find the certificate to verify the private key JWT signature.
// https://datatracker.ietf.org/doc/html/rfc7515#section-4.1.7
func hashKeyID(cert *x509.Certificate, header structs.OIDCClientAssertionKeyIDHeader) (string, error) {
	var hasher hash.Hash
	switch header {
	case structs.OIDCClientAssertionHeaderX5t:
		hasher = sha1.New()
	case structs.OIDCClientAssertionHeaderX5tS256:
		hasher = sha256.New()
	default:
		// this should be validated long before here, at upsert
		return "", fmt.Errorf(`%w; must be one of: "x5t", "x5t#S256"`, structs.ErrInvalidKeyIDHeader)
	}
	hasher.Write(cert.Raw)
	hashed := hasher.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(hashed), nil
}

var (
	// for user convenience, they may exclude key/cert pem header/footer
	beginPrivateKey  = []byte("-----BEGIN PRIVATE KEY-----")
	endPrivateKey    = []byte("-----END PRIVATE KEY-----")
	beginCertificate = []byte("-----BEGIN CERTIFICATE-----")
	endCertificate   = []byte("-----END CERTIFICATE-----")
)

// wrapBeginEnd wraps the provided bts in "{begin}\n{bts}\n{end}".
// if begin and end are already there, it ensures "\n" is between them and bts.
func wrapBeginEnd(bts, begin, end []byte) []byte {
	cp := bytes.Clone(bts)
	cp = bytes.TrimPrefix(cp, begin)
	cp = bytes.TrimSuffix(cp, end)
	cp = bytes.TrimSpace(cp)
	cp = bytes.Join([][]byte{begin, cp, end}, []byte("\n"))
	return cp
}

package oidc

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	gojwt "github.com/golang-jwt/jwt/v5"
	cass "github.com/hashicorp/cap/oidc/clientassertion"
	"github.com/hashicorp/nomad/nomad/structs"
)

type ActiveKeyGetter interface {
	GetActiveKey() (key *rsa.PrivateKey, id string, err error)
}

func BuildClientAssertionJWT(config *structs.ACLAuthMethodConfig, keys ActiveKeyGetter) (*cass.JWT, error) {
	// should already be validated by caller, but just in case.
	if config == nil || config.OIDCClientAssertion == nil {
		return nil, errors.New("no auth method config or client assertion")
	}

	// this is all we use config for
	clientID := config.OIDCClientID
	verbose := config.VerboseLogging
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
		opts = append(opts,
			cass.WithClientSecret(as.ClientSecret(), algo))

	case structs.OIDCKeySourceNomad:
		algo := cass.RS256
		key, keyID, err := keys.GetActiveKey()
		if err != nil {
			return nil, err
		}
		opts = append(opts,
			cass.WithRSAKey(key, algo),
			cass.WithKeyID(keyID),
		)

	case structs.OIDCKeySourcePrivateKey:
		algo := cass.RSAlgorithm(as.KeyAlgorithm)
		rsaKey, err := getCassPrivateKey(as.PrivateKey)
		if err != nil {
			return nil, err
		}
		keyID := as.PrivateKey.KeyID
		if keyID == "" {
			cert, err := getCassCert(as.PrivateKey)
			if err != nil {
				return nil, err
			}
			keyID = X5T(cert)
		}
		opts = append(opts,
			cass.WithRSAKey(rsaKey, algo),
			cass.WithKeyID(keyID),
		)

	default: // this shouldn't happen, but just in case
		return nil, fmt.Errorf("unknown OIDC KeySource %q", as.KeySource)
	}

	j, err := cass.NewJWT(clientID, as.Audience, opts...)
	if err != nil {
		return nil, err
	}
	if verbose {
		// TODO: hmmm...
		//signed, _ := j.Serialize()
		//a.logger.Debug("approximate signed jwt for client assertion", "jwt", signed)
	}
	return j, nil
}

// getCassPrivateKey parses the structs.OIDCClientAssertionKey PemKeyFile
// or Base64PemKey, depending on which is set.
func getCassPrivateKey(k *structs.OIDCClientAssertionKey) (key *rsa.PrivateKey, err error) {
	var bts []byte
	var source string // for informative error messages

	// file on disk
	if k.PemKeyFile != "" {
		source = "PemKeyFile"
		bts, err = os.ReadFile(k.PemKeyFile)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", source, err)
		}
	}
	// or base64 string
	if k.Base64PemKey != "" {
		source = "Base64PemKey"
		bts = make([]byte, base64.StdEncoding.DecodedLen(len(k.Base64PemKey)))
		_, err := base64.StdEncoding.Decode(bts, []byte(k.Base64PemKey))
		if err != nil {
			return nil, fmt.Errorf("error decoding %s: %w", source, err)
		}
	}

	key, err = gojwt.ParseRSAPrivateKeyFromPEM(bts)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", source, err)
	}
	return key, nil
}

// getCassCert parses the structs.OIDCClientAssertionKey PemCertFile
// or Base64PemCert, depending on which is set.
func getCassCert(k *structs.OIDCClientAssertionKey) (*x509.Certificate, error) {
	var bts []byte
	var err error
	var source string // for informative error messages

	// file on disk
	if k.PemCertFile != "" {
		source = "PemCertFile"
		bts, err = os.ReadFile(k.PemCertFile)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", source, err)
		}
	}
	// or base64 string
	if k.Base64PemCert != "" {
		source = "Base64PemCert"
		bts = make([]byte, base64.StdEncoding.DecodedLen(len(k.Base64PemCert)))
		_, err := base64.StdEncoding.Decode(bts, []byte(k.Base64PemCert))
		if err != nil {
			return nil, fmt.Errorf("error decoding %s: %w", source, err)
		}
	}

	block, _ := pem.Decode(bts)
	if block == nil {
		return nil, fmt.Errorf("failed to decode %s PEM block", source)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s bytes: %w", source, err)
	}
	return cert, nil
}

// X5T parses the certificate to an "x5t" header to set as the key ID.
// https://datatracker.ietf.org/doc/html/rfc7515#section-4.1.7
func X5T(cert *x509.Certificate) string {
	hasher := sha1.New()
	hasher.Write(cert.Raw)
	hashed := hasher.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(hashed)
}

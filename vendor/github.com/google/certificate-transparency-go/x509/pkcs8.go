// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/x509/pkix"
	"golang.org/x/crypto/ed25519"
)

// pkcs8 reflects an ASN.1, PKCS#8 PrivateKey. See
// ftp://ftp.rsasecurity.com/pub/pkcs/pkcs-8/pkcs-8v1_2.asn
// and RFC 5208.
type pkcs8 struct {
	Version    int
	Algo       pkix.AlgorithmIdentifier
	PrivateKey []byte
	// optional attributes omitted.
}

// ParsePKCS8PrivateKey parses an unencrypted, PKCS#8 private key.
// See RFC 5208.
func ParsePKCS8PrivateKey(der []byte) (key interface{}, err error) {
	var privKey pkcs8
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		return nil, err
	}
	switch {
	case privKey.Algo.Algorithm.Equal(OIDPublicKeyRSA):
		key, err = ParsePKCS1PrivateKey(privKey.PrivateKey)
		if err != nil {
			return nil, errors.New("x509: failed to parse RSA private key embedded in PKCS#8: " + err.Error())
		}
		return key, nil

	case privKey.Algo.Algorithm.Equal(OIDPublicKeyECDSA):
		bytes := privKey.Algo.Parameters.FullBytes
		namedCurveOID := new(asn1.ObjectIdentifier)
		if _, err := asn1.Unmarshal(bytes, namedCurveOID); err != nil {
			namedCurveOID = nil
		}
		key, err = parseECPrivateKey(namedCurveOID, privKey.PrivateKey)
		if err != nil {
			return nil, errors.New("x509: failed to parse EC private key embedded in PKCS#8: " + err.Error())
		}
		return key, nil

	case privKey.Algo.Algorithm.Equal(OIDPublicKeyEd25519):
		key, err = ParseEd25519PrivateKey(privKey.PrivateKey)
		if err != nil {
			return nil, errors.New("x509: failed to parse Ed25519 private key embedded in PKCS#8: " + err.Error())
		}
		return key, nil

	default:
		return nil, fmt.Errorf("x509: PKCS#8 wrapping contained private key with unknown algorithm: %v", privKey.Algo.Algorithm)
	}
}

// MarshalPKCS8PrivateKey converts a private key to PKCS#8 encoded form.
// The following key types are supported: *rsa.PrivateKey, *ecdsa.PrivateKey.
// Unsupported key types result in an error.
//
// See RFC 5208.
func MarshalPKCS8PrivateKey(key interface{}) ([]byte, error) {
	var privKey pkcs8

	switch k := key.(type) {
	case *rsa.PrivateKey:
		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm:  OIDPublicKeyRSA,
			Parameters: asn1.NullRawValue,
		}
		privKey.PrivateKey = MarshalPKCS1PrivateKey(k)

	case *ecdsa.PrivateKey:
		oid, ok := OIDFromNamedCurve(k.Curve)
		if !ok {
			return nil, errors.New("x509: unknown curve while marshalling to PKCS#8")
		}

		oidBytes, err := asn1.Marshal(oid)
		if err != nil {
			return nil, errors.New("x509: failed to marshal curve OID: " + err.Error())
		}

		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm: OIDPublicKeyECDSA,
			Parameters: asn1.RawValue{
				FullBytes: oidBytes,
			},
		}

		if privKey.PrivateKey, err = marshalECPrivateKeyWithOID(k, nil); err != nil {
			return nil, errors.New("x509: failed to marshal EC private key while building PKCS#8: " + err.Error())
		}

	case ed25519.PrivateKey:
		privKey.Algo = pkix.AlgorithmIdentifier{Algorithm: OIDPublicKeyEd25519}
		var err error
		if privKey.PrivateKey, err = MarshalEd25519PrivateKey(k); err != nil {
			return nil, fmt.Errorf("x509: failed to marshal Ed25519 private key while building PKCS#8: %v", err)
		}

	default:
		return nil, fmt.Errorf("x509: unknown key type while marshalling PKCS#8: %T", key)
	}

	return asn1.Marshal(privKey)
}

// MarshalEd25519PrivateKey converts an Ed25519 private key to ASN.1 DER encoded form
// (as an OCTET STRING holding the key seed, as per RFC 8410 section 7).
func MarshalEd25519PrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	return asn1.Marshal(key.Seed())
}

// ParseEd25519PrivateKey returns an Ed25519 private key from its ASN.1 DER encoded form
// (as an OCTET STRING holding the key seed, as per RFC 8410 section 7).
func ParseEd25519PrivateKey(der []byte) (ed25519.PrivateKey, error) {
	var keySeed []byte
	if _, err := asn1.Unmarshal(der, &keySeed); err != nil {
		return nil, err
	}
	if len(keySeed) != ed25519.SeedSize {
		return nil, fmt.Errorf("x509: ed25519 seed length should be %d bytes, got %d", ed25519.SeedSize, len(keySeed))
	}
	return ed25519.NewKeyFromSeed(keySeed), nil
}

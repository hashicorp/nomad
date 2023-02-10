package jwt

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/cap/jwt"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/slices"
)

// Validate performs token signature verification and returns a list of claims
func Validate(ctx context.Context, token string, methodConf *structs.ACLAuthMethodConfig) (map[string]any, error) {
	var keySet jwt.KeySet
	var err error

	// JWT validation can happen in 3 ways:
	// - via embedded public keys, locally
	// - via JWKS
	// - or via OIDC provider
	if len(methodConf.JWTValidationPubKeys) != 0 {
		keySet, err = usingStaticKeys(methodConf.JWTValidationPubKeys)
		if err != nil {
			return nil, err
		}
	} else if methodConf.JWKSURL != "" {
		keySet, err = usingJWKS(ctx, methodConf.JWKSURL, methodConf.JWKSCACert)
		if err != nil {
			return nil, err
		}
	} else if methodConf.OIDCDiscoveryURL != "" {
		keySet, err = usingOIDC(ctx, methodConf.OIDCDiscoveryURL, methodConf.DiscoveryCaPem)
		if err != nil {
			return nil, err
		}
	}

	var algorithms []jwt.Alg
	for _, m := range methodConf.SigningAlgs {
		alg := jwt.Alg(m)
		algorithms = append(algorithms, alg)
	}

	expected := jwt.Expected{
		Audiences:         methodConf.BoundAudiences,
		SigningAlgorithms: algorithms,
		NotBeforeLeeway:   methodConf.NotBeforeLeeway,
		ExpirationLeeway:  methodConf.ExpirationLeeway,
		ClockSkewLeeway:   methodConf.ClockSkewLeeway,
	}

	validator, err := jwt.NewValidator(keySet)
	if err != nil {
		return nil, err
	}

	claims, err := validator.Validate(ctx, token, expected)
	if err != nil {
		return nil, fmt.Errorf("unable to verify signature of JWT token: %v", err)
	}

	// validate issuer manually, because we allow users to specify an array
	if len(methodConf.BoundIssuer) > 0 {
		if _, ok := claims["iss"]; !ok {
			return nil, fmt.Errorf(
				"auth method specifies BoundIssuers but the provided token does not contain issuer information",
			)
		}
		var iss string
		var ok bool
		if iss, ok = claims["iss"].(string); !ok {
			return nil, fmt.Errorf("unable to read iss property of provided token")
		}
		if !slices.Contains(methodConf.BoundIssuer, iss) {
			return nil, fmt.Errorf("invalid JWT issuer: %v", claims["iss"])
		}
	}

	return claims, nil
}

func usingStaticKeys(keys []string) (jwt.KeySet, error) {
	var parsedKeys []crypto.PublicKey
	for _, v := range keys {
		key, err := jwt.ParsePublicKeyPEM([]byte(v))
		parsedKeys = append(parsedKeys, key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse public key for JWT auth: %v", err)
		}
	}
	return jwt.NewStaticKeySet(parsedKeys)
}

func usingJWKS(ctx context.Context, jwksurl, jwkscapem string) (jwt.KeySet, error) {
	// Measure the JWKS endpoint performance.
	defer metrics.MeasureSince([]string{"nomad", "acl", "jwks"}, time.Now())

	keySet, err := jwt.NewJSONWebKeySet(ctx, jwksurl, jwkscapem)
	if err != nil {
		return nil, fmt.Errorf("unable to get validation keys from JWKS: %v", err)
	}
	return keySet, nil
}

func usingOIDC(ctx context.Context, oidcurl string, oidccapem []string) (jwt.KeySet, error) {
	// Measure the OIDC endpoint performance.
	defer metrics.MeasureSince([]string{"nomad", "acl", "oidc_jwt"}, time.Now())

	// TODO why do we have DiscoverCaPem as an array but JWKSCaPem as a single string?
	pem := ""
	if len(oidccapem) > 0 {
		pem = oidccapem[0]
	}

	keySet, err := jwt.NewOIDCDiscoveryKeySet(ctx, oidcurl, pem)
	if err != nil {
		return nil, fmt.Errorf("unable to get validation keys from OIDC provider: %v", err)
	}
	return keySet, nil
}

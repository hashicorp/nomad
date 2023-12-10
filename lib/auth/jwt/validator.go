// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jwt

import (
	"context"
	"crypto"
	"fmt"
	"slices"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/cap/jwt"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Validate performs token signature verification and JWT header validation,
// and returns a list of claims or an error in case any validation or signature
// verification fails.
func Validate(ctx context.Context, token string, methodConf *structs.ACLAuthMethodConfig) (map[string]any, error) {
	var (
		keySet jwt.KeySet
		err    error
	)

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

	// SigningAlgs field is a string, we need to convert it to a type the go-jwt
	// accepts in order to validate.
	toAlgFn := func(m string) jwt.Alg { return jwt.Alg(m) }
	algorithms := helper.ConvertSlice(methodConf.SigningAlgs, toAlgFn)

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
		if iss, ok := claims["iss"].(string); !ok {
			return nil, fmt.Errorf("unable to read iss property of provided token")
		} else if !slices.Contains(methodConf.BoundIssuer, iss) {
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
	defer metrics.MeasureSince([]string{"nomad", "acl", "jwt", "jwks"}, time.Now())

	keySet, err := jwt.NewJSONWebKeySet(ctx, jwksurl, jwkscapem)
	if err != nil {
		return nil, fmt.Errorf("unable to get validation keys from JWKS: %v", err)
	}
	return keySet, nil
}

func usingOIDC(ctx context.Context, oidcurl string, oidccapem []string) (jwt.KeySet, error) {
	// Measure the OIDC endpoint performance.
	defer metrics.MeasureSince([]string{"nomad", "acl", "jwt", "oidc_jwt"}, time.Now())

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

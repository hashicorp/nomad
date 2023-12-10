// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package joseutil

import (
	"errors"

	"github.com/go-jose/go-jose/v3/jwt"
)

var ErrNoKeyID = errors.New("missing key ID header")

// KeyID returns the KeyID header for a JWT or ErrNoKeyID if a key id could not
// be found. No clue why jose makes this so awkward.
func KeyID(token *jwt.JSONWebToken) (string, error) {
	for _, h := range token.Headers {
		if h.KeyID != "" {
			return h.KeyID, nil
		}
	}
	return "", ErrNoKeyID
}

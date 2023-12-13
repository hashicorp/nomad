// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package crypto

import (
	"errors"
	"fmt"

	// note: this is aliased so that it's more noticeable if someone
	// accidentally swaps it out for math/rand via running goimports
	cryptorand "crypto/rand"
)

// Bytes gets a slice of cryptographically random bytes of the given length and
// enforces that we check for short reads to avoid entropy exhaustion.
func Bytes(length int) ([]byte, error) {
	key := make([]byte, length)
	n, err := cryptorand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("could not read from random source: %v", err)
	}
	if n < length {
		return nil, errors.New("entropy exhausted")
	}
	return key, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package uuid

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/crypto"
)

// Generate is used to generate a random UUID.
func Generate() string {
	buf, err := crypto.Bytes(16)
	if err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

// Short is used to generate the first 8 characters of a UUID.
func Short() string {
	return Generate()[0:8]
}

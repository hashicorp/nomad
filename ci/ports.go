// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci

import (
	"fmt"

	"github.com/shoenig/test/portal"
)

type fatalTester struct{}

func (t *fatalTester) Fatalf(msg string, args ...any) {
	panic(fmt.Sprintf(msg, args...))
}

// PortAllocator is used to acquire unused ports for testing real network
// listeners.
var PortAllocator = portal.New(
	new(fatalTester),
	portal.WithAddress("127.0.0.1"),
)

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

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

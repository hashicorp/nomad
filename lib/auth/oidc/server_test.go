// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestCallbackServer(t *testing.T) {

	testCallbackServer, err := NewCallbackServer("localhost:4649")
	must.NoError(t, err)
	must.NotNil(t, testCallbackServer)

	defer func() {
		must.NoError(t, testCallbackServer.Close())
	}()
	must.StrNotEqFold(t, "", testCallbackServer.Nonce())
	must.Eq(t, "http://localhost:4649/oidc/callback", testCallbackServer.RedirectURI())
}

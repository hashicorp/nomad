// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/serviceregistration/nsd"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/shoenig/test/must"
)

var (
	_ NodeIdentityHandler = (*widmgr.Signer)(nil)
	_ NodeIdentityHandler = (*nsd.ServiceRegistrationHandler)(nil)
)

func Test_assertAndSetNodeIdentityToken(t *testing.T) {
	ci.Parallel(t)

	// Call the function with a non-nil object that implements the interface and
	// verify that SetNodeIdentityToken is called with the expected token.
	testImpl := &testHandler{}
	assertAndSetNodeIdentityToken(testImpl, "test-token")
	must.Eq(t, "test-token", testImpl.t)
}

type testHandler struct{ t string }

func (t *testHandler) SetNodeIdentityToken(token string) { t.t = token }

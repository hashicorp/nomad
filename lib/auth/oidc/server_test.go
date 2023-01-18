package oidc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestCallbackServer(t *testing.T) {

	testCallbackServer, err := NewCallbackServer("127.0.0.1:4649")
	must.NoError(t, err)
	must.NotNil(t, testCallbackServer)

	defer func() {
		must.NoError(t, testCallbackServer.Close())
	}()
	must.StrNotEqFold(t, "", testCallbackServer.Nonce())
	must.StrNotEqFold(t, "", testCallbackServer.RedirectURI())
}

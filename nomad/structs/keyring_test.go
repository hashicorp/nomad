package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestKeyring_OIDCDiscoveryConfig(t *testing.T) {
	ci.Parallel(t)

	c, err := NewOIDCDiscoveryConfig("")
	must.Error(t, err)
	must.Nil(t, c)

	c, err = NewOIDCDiscoveryConfig(":/invalid")
	must.Error(t, err)
	must.Nil(t, c)

	const testIssuer = "https://oidc.test.nomadproject.io/"
	c, err = NewOIDCDiscoveryConfig(testIssuer)
	must.NoError(t, err)
	must.NotNil(t, c)
	must.Eq(t, testIssuer, c.Issuer)
	must.StrHasPrefix(t, testIssuer, c.JWKS)
	must.SliceNotEmpty(t, c.IDTokenAlgs)
	must.SliceNotEmpty(t, c.ResponseTypes)
	must.SliceNotEmpty(t, c.Subjects)
}

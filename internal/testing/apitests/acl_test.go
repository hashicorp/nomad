// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"net/url"
	"testing"
	"time"

	capOIDC "github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestACLOIDC_GetAuthURL(t *testing.T) {
	ci.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	// Set up the test OIDC provider.
	oidcTestProvider := capOIDC.StartTestProvider(t)
	defer oidcTestProvider.Stop()
	oidcTestProvider.SetAllowedRedirectURIs([]string{"http://127.0.0.1:4649/oidc/callback"})

	// Generate and upsert an ACL auth method for use. Certain values must be
	// taken from the cap OIDC provider just like real world use.
	mockedAuthMethod := api.ACLAuthMethod{
		Name:          "api-test-auth-method",
		Type:          api.ACLAuthMethodTypeOIDC,
		TokenLocality: api.ACLAuthMethodTokenLocalityGlobal,
		MaxTokenTTL:   10 * time.Hour,
		Default:       true,
		Config: &api.ACLAuthMethodConfig{
			OIDCDiscoveryURL:    oidcTestProvider.Addr(),
			OIDCClientID:        "mock",
			OIDCClientSecret:    "verysecretsecret",
			BoundAudiences:      []string{"mock"},
			AllowedRedirectURIs: []string{"http://127.0.0.1:4649/oidc/callback"},
			DiscoveryCaPem:      []string{oidcTestProvider.CACert()},
			SigningAlgs:         []string{"ES256"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
	}

	createdAuthMethod, writeMeta, err := testClient.ACLAuthMethods().Create(&mockedAuthMethod, nil)
	must.NoError(t, err)
	must.NotNil(t, createdAuthMethod)
	assertWriteMeta(t, writeMeta)

	// Generate and make the request.
	authURLRequest := api.ACLOIDCAuthURLRequest{
		AuthMethodName: createdAuthMethod.Name,
		RedirectURI:    createdAuthMethod.Config.AllowedRedirectURIs[0],
		ClientNonce:    "fpSPuaodKevKfDU3IeXb",
	}

	authURLResp, _, err := testClient.ACLAuth().GetAuthURL(&authURLRequest, nil)
	must.NoError(t, err)

	// The response URL comes encoded, so decode this and check we have each
	// component we expect.
	escapedURL, err := url.PathUnescape(authURLResp.AuthURL)
	must.NoError(t, err)
	must.StrContains(t, escapedURL, "/authorize?client_id=mock")
	must.StrContains(t, escapedURL, "&nonce=fpSPuaodKevKfDU3IeXb")
	must.StrContains(t, escapedURL, "&redirect_uri=http://127.0.0.1:4649/oidc/callback")
	must.StrContains(t, escapedURL, "&response_type=code")
	must.StrContains(t, escapedURL, "&scope=openid")
	must.StrContains(t, escapedURL, "&state=st_")
}

func TestACLOIDC_CompleteAuth(t *testing.T) {
	ci.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	// Set up the test OIDC provider.
	oidcTestProvider := capOIDC.StartTestProvider(t)
	defer oidcTestProvider.Stop()
	oidcTestProvider.SetAllowedRedirectURIs([]string{"http://127.0.0.1:4649/oidc/callback"})

	// Generate and upsert an ACL auth method for use. Certain values must be
	// taken from the cap OIDC provider just like real world use.
	mockedAuthMethod := api.ACLAuthMethod{
		Name:          "api-test-auth-method",
		Type:          api.ACLAuthMethodTypeOIDC,
		TokenLocality: api.ACLAuthMethodTokenLocalityGlobal,
		MaxTokenTTL:   10 * time.Hour,
		Default:       true,
		Config: &api.ACLAuthMethodConfig{
			OIDCDiscoveryURL:    oidcTestProvider.Addr(),
			OIDCClientID:        "mock",
			OIDCClientSecret:    "verysecretsecret",
			BoundAudiences:      []string{"mock"},
			AllowedRedirectURIs: []string{"http://127.0.0.1:4649/oidc/callback"},
			DiscoveryCaPem:      []string{oidcTestProvider.CACert()},
			SigningAlgs:         []string{"ES256"},
			ClaimMappings:       map[string]string{},
			ListClaimMappings: map[string]string{
				"http://nomad.internal/roles":    "roles",
				"http://nomad.internal/policies": "policies",
			},
		},
	}

	createdAuthMethod, writeMeta, err := testClient.ACLAuthMethods().Create(&mockedAuthMethod, nil)
	must.NoError(t, err)
	must.NotNil(t, createdAuthMethod)
	assertWriteMeta(t, writeMeta)

	// Set our custom data and some expected values, so we can make the call
	// and use the test provider.
	oidcTestProvider.SetExpectedAuthNonce("fpSPuaodKevKfDU3IeXb")
	oidcTestProvider.SetExpectedAuthCode("codeABC")
	oidcTestProvider.SetCustomAudience("mock")
	oidcTestProvider.SetExpectedState("st_someweirdstateid")
	oidcTestProvider.SetCustomClaims(map[string]interface{}{
		"azp":                            "mock",
		"http://nomad.internal/policies": []string{"engineering"},
		"http://nomad.internal/roles":    []string{"engineering"},
	})

	// Upsert an ACL policy and role, so that we can reference this within our
	// OIDC claims.
	mockedACLPolicy := api.ACLPolicy{
		Name:  "api-oidc-login-test",
		Rules: `namespace "default" { policy = "write"}`,
	}
	_, err = testClient.ACLPolicies().Upsert(&mockedACLPolicy, nil)
	must.NoError(t, err)

	mockedACLRole := api.ACLRole{
		Name:     "api-oidc-login-test",
		Policies: []*api.ACLRolePolicyLink{{Name: mockedACLPolicy.Name}},
	}
	createRoleResp, _, err := testClient.ACLRoles().Create(&mockedACLRole, nil)
	must.NoError(t, err)
	must.NotNil(t, createRoleResp)

	// Generate and upsert two binding rules, so we can test both ACL Policy
	// and Role claim mapping.
	mockedBindingRule1 := api.ACLBindingRule{
		AuthMethod: mockedAuthMethod.Name,
		Selector:   "engineering in list.policies",
		BindType:   api.ACLBindingRuleBindTypePolicy,
		BindName:   mockedACLPolicy.Name,
	}
	createBindingRole1Resp, _, err := testClient.ACLBindingRules().Create(&mockedBindingRule1, nil)
	must.NoError(t, err)
	must.NotNil(t, createBindingRole1Resp)

	mockedBindingRule2 := api.ACLBindingRule{
		AuthMethod: mockedAuthMethod.Name,
		Selector:   "engineering in list.roles",
		BindType:   api.ACLBindingRuleBindTypeRole,
		BindName:   mockedACLRole.Name,
	}
	createBindingRole2Resp, _, err := testClient.ACLBindingRules().Create(&mockedBindingRule2, nil)
	must.NoError(t, err)
	must.NotNil(t, createBindingRole2Resp)

	// Generate and make the request.
	authURLRequest := api.ACLOIDCCompleteAuthRequest{
		AuthMethodName: createdAuthMethod.Name,
		RedirectURI:    createdAuthMethod.Config.AllowedRedirectURIs[0],
		ClientNonce:    "fpSPuaodKevKfDU3IeXb",
		State:          "st_someweirdstateid",
		Code:           "codeABC",
	}

	completeAuthResp, _, err := testClient.ACLAuth().CompleteAuth(&authURLRequest, nil)
	must.NoError(t, err)
	must.NotNil(t, completeAuthResp)
	must.Len(t, 1, completeAuthResp.Policies)
	must.Eq(t, mockedACLPolicy.Name, completeAuthResp.Policies[0])
	must.Len(t, 1, completeAuthResp.Roles)
	must.Eq(t, mockedACLRole.Name, completeAuthResp.Roles[0].Name)
	must.Eq(t, createRoleResp.ID, completeAuthResp.Roles[0].ID)
}

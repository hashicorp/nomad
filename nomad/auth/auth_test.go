// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestAuthenticateDefault(t *testing.T) {
	ci.Parallel(t)

	testAuthenticator := func(t *testing.T, store *state.StateStore,
		hasACLs, verifyTLS bool) *Authenticator {
		leaderACL := uuid.Generate()
		return NewAuthenticator(&AuthenticatorConfig{
			StateFn:        func() *state.StateStore { return store },
			Logger:         testlog.HCLogger(t),
			GetLeaderACLFn: func() string { return leaderACL },
			AclsEnabled:    hasACLs,
			VerifyTLS:      verifyTLS,
			Region:         "global",
			Encrypter:      newTestEncrypter(),
		})
	}

	testCases := []struct {
		name   string
		testFn func(*testing.T, *state.StateStore)
	}{
		{
			name: "mTLS and ACLs but anonymous",
			testFn: func(t *testing.T, store *state.StateStore) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = ""

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "no mTLS or ACLs but anonymous",
			testFn: func(t *testing.T, store *state.StateStore) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = ""

				auth := testAuthenticator(t, store, false, false)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "token:acls-disabled", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, acl.ACLsDisabledACL, aclObj)
				must.True(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS and ACLs but anonymous with TLS context",
			testFn: func(t *testing.T, store *state.StateStore) {
				ctx := newTestContext(t, "cli.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = ""

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "cli.global.nomad:192.168.1.1", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS and ACLs with client secret",
			testFn: func(t *testing.T, store *state.StateStore) {
				node := mock.Node()
				store.UpsertNode(structs.MsgTypeTestSetup, 100, node)

				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = node.SecretID

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "client:"+node.ID, args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.True(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "mTLS and ACLs with invalid token no TLS context",
			testFn: func(t *testing.T, store *state.StateStore) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = uuid.Generate()

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS and ACLs with invalid token",
			testFn: func(t *testing.T, store *state.StateStore) {
				ctx := newTestContext(t, "cli.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = uuid.Generate()

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS and ACLs with valid ACL token",
			testFn: func(t *testing.T, store *state.StateStore) {

				token1 := mock.ACLToken()
				store.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{
					token1,
				})

				ctx := newTestContext(t, "cli.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = token1.SecretID

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "token:"+token1.AccessorID, args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead()) // no permissions
			},
		},
		{
			name: "mTLS and ACLs with expired ACL token",
			testFn: func(t *testing.T, store *state.StateStore) {
				token2 := mock.ACLToken()
				expireTime := time.Now().Add(time.Second * -10)
				token2.ExpirationTime = &expireTime
				store.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{
					token2,
				})

				ctx := newTestContext(t, "cli.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = token2.SecretID

				auth := testAuthenticator(t, store, true, true)

				err := auth.Authenticate(ctx, args)
				must.ErrorIs(t, err, structs.ErrTokenExpired)
				must.Eq(t, "unauthenticated", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS but no ACLs with valid ACL token",
			testFn: func(t *testing.T, store *state.StateStore) {

				token3 := mock.ACLToken()
				store.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{
					token3,
				})

				ctx := newTestContext(t, "cli.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = token3.SecretID

				auth := testAuthenticator(t, store, false, true)

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "token:acls-disabled", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, acl.ACLsDisabledACL, aclObj)
				must.True(t, aclObj.AllowAgentRead())
			},
		},
		{
			name: "mTLS and ACLs with valid WI token",
			testFn: func(t *testing.T, store *state.StateStore) {
				alloc := mock.Alloc()
				alloc.ClientStatus = structs.AllocClientStatusRunning
				task := alloc.LookupTask("web")
				identity := task.Identity
				wih := task.IdentityHandle(identity)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				claims := structs.NewIdentityClaims(alloc.Job, alloc, wih, identity, time.Now())

				auth := testAuthenticator(t, store, true, true)
				token, err := auth.encrypter.(*testEncrypter).signClaim(claims)
				must.NoError(t, err)

				ctx := newTestContext(t, "client.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = token

				err = auth.Authenticate(ctx, args)
				must.EqError(t, err, "allocation does not exist")

				// insert alloc so it's live
				store.UpsertAllocs(structs.MsgTypeTestSetup, 200,
					[]*structs.Allocation{alloc})

				args = &structs.GenericRequest{}
				args.AuthToken = token
				err = auth.Authenticate(ctx, args)
				must.NoError(t, err)

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.False(t, aclObj.AllowAgentRead())
				must.True(t,
					aclObj.AllowServiceRegistrationReadList(alloc.Job.Namespace, true))
				must.Eq(t, "alloc:"+alloc.ID, args.GetIdentity().String())

				// alloc becomes terminal
				alloc.ClientStatus = structs.AllocClientStatusComplete
				store.UpsertAllocs(structs.MsgTypeTestSetup, 200,
					[]*structs.Allocation{alloc})

				args = &structs.GenericRequest{}
				args.AuthToken = token
				err = auth.Authenticate(ctx, args)
				must.EqError(t, err, "allocation is terminal")
				must.Eq(t, "unauthenticated", args.GetIdentity().String())

				aclObj, err = auth.ResolveACL(args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Nil(t, aclObj)
				must.False(t,
					aclObj.AllowServiceRegistrationReadList(alloc.Job.Namespace, true))

			},
		},
		{
			name: "mTLS and ACLs with invalid WI token",
			testFn: func(t *testing.T, store *state.StateStore) {
				alloc := mock.Alloc()
				task := alloc.LookupTask("web")
				identity := task.Identity
				wih := task.IdentityHandle(identity)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				claims := structs.NewIdentityClaims(alloc.Job, alloc, wih, identity, time.Now())

				auth := testAuthenticator(t, store, true, true)
				token, err := auth.encrypter.(*testEncrypter).signClaim(claims)
				must.NoError(t, err)

				// break the token
				token = strings.ReplaceAll(token, "0", "1")
				ctx := newTestContext(t, "client.nomad.global", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = token

				err = auth.Authenticate(ctx, args)
				must.ErrorContains(t, err, "invalid signature")

				aclObj, err := auth.ResolveACL(args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Nil(t, aclObj)
				must.False(t,
					aclObj.AllowServiceRegistrationReadList(alloc.Job.Namespace, true))
			},
		},
		{
			name: "mTLS and ACLs from static handler with leader ACL token",
			testFn: func(t *testing.T, store *state.StateStore) {

				auth := testAuthenticator(t, store, true, true)

				args := &structs.GenericRequest{}
				args.AuthToken = auth.getLeaderACL()
				var ctx *testContext

				err := auth.Authenticate(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "token:leader", args.GetIdentity().String())

				aclObj, err := auth.ResolveACL(args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.True(t, aclObj.IsManagement())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := testStateStore(t)
			tc.testFn(t, store)
		})
	}
}

func TestAuthenticateServerOnly(t *testing.T) {
	ci.Parallel(t)

	testAuthenticator := func(t *testing.T, store *state.StateStore,
		hasACLs, verifyTLS bool) *Authenticator {
		leaderACL := uuid.Generate()
		return NewAuthenticator(&AuthenticatorConfig{
			StateFn:        func() *state.StateStore { return store },
			Logger:         testlog.HCLogger(t),
			GetLeaderACLFn: func() string { return leaderACL },
			AclsEnabled:    hasACLs,
			VerifyTLS:      verifyTLS,
			Region:         "global",
			Encrypter:      nil,
		})
	}

	testCases := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "no mTLS",
			testFn: func(t *testing.T) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}

				store := testStateStore(t)
				auth := testAuthenticator(t, store, true, false)

				aclObj, err := auth.AuthenticateServerOnly(ctx, args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())
				must.True(t, aclObj.AllowServerOp())
			},
		},
		{
			name: "no mTLS but client cert",
			testFn: func(t *testing.T) {
				ctx := newTestContext(t, "client.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}

				store := testStateStore(t)
				auth := testAuthenticator(t, store, true, false)

				aclObj, err := auth.AuthenticateServerOnly(ctx, args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())
				must.True(t, aclObj.AllowServerOp())
			},
		},
		{
			name: "with mTLS but client cert",
			testFn: func(t *testing.T) {
				ctx := newTestContext(t, "client.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}

				store := testStateStore(t)
				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateServerOnly(ctx, args)
				must.EqError(t, err,
					"invalid certificate: client.global.nomad not in expected server.global.nomad")
				must.Eq(t, "client.global.nomad:192.168.1.1", args.GetIdentity().String())
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowServerOp())
			},
		},
		{
			name: "with mTLS and server cert",
			testFn: func(t *testing.T) {
				ctx := newTestContext(t, "server.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}

				store := testStateStore(t)
				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateServerOnly(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "server.global.nomad:192.168.1.1", args.GetIdentity().String())
				must.NotNil(t, aclObj)
				must.True(t, aclObj.AllowServerOp())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(t)
		})
	}
}

func TestAuthenticateClientOnly(t *testing.T) {
	ci.Parallel(t)

	testAuthenticator := func(t *testing.T, store *state.StateStore,
		hasACLs, verifyTLS bool) *Authenticator {
		leaderACL := uuid.Generate()

		return NewAuthenticator(&AuthenticatorConfig{
			StateFn:        func() *state.StateStore { return store },
			Logger:         testlog.HCLogger(t),
			GetLeaderACLFn: func() string { return leaderACL },
			AclsEnabled:    hasACLs,
			VerifyTLS:      verifyTLS,
			Region:         "global",
			Encrypter:      nil,
		})
	}

	testCases := []struct {
		name   string
		testFn func(*testing.T, *state.StateStore, *structs.Node)
	}{
		{
			name: "no mTLS or ACLs but no node secret",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = ""

				auth := testAuthenticator(t, store, false, false)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "no mTLS or ACLs but with node secret",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = node.SecretID

				auth := testAuthenticator(t, store, false, false)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, "client:"+node.ID, args.GetIdentity().String())
				must.True(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "no mTLS but with ACLs",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = node.SecretID

				auth := testAuthenticator(t, store, true, false)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.NoError(t, err)
				must.NotNil(t, aclObj)
				must.Eq(t, "client:"+node.ID, args.GetIdentity().String())
				must.True(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "no mTLS but with ACLs and bad secret",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, noTLSCtx, "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = uuid.Generate()

				auth := testAuthenticator(t, store, true, false)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Eq(t, ":192.168.1.1", args.GetIdentity().String())
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "with mTLS and ACLs but CLI cert",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, "cli.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}

				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.EqError(t, err,
					"invalid certificate: cli.global.nomad not in expected client.global.nomad, server.global.nomad")
				must.Eq(t, "cli.global.nomad:192.168.1.1", args.GetIdentity().String())
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "with mTLS and ACLs with server cert but bad token",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, "server.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = uuid.Generate()

				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.ErrorIs(t, err, structs.ErrPermissionDenied)
				must.Eq(t, "server.global.nomad:192.168.1.1", args.GetIdentity().String())
				must.Nil(t, aclObj)
				must.False(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "with mTLS and ACLs with server cert and valid token",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, "server.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = node.SecretID

				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.NoError(t, err)

				must.Eq(t, "client:"+node.ID, args.GetIdentity().String())
				must.NotNil(t, aclObj)
				must.True(t, aclObj.AllowClientOp())
			},
		},
		{
			name: "with mTLS and ACLs with client cert",
			testFn: func(t *testing.T, store *state.StateStore, node *structs.Node) {
				ctx := newTestContext(t, "client.global.nomad", "192.168.1.1")
				args := &structs.GenericRequest{}
				args.AuthToken = node.SecretID

				auth := testAuthenticator(t, store, true, true)

				aclObj, err := auth.AuthenticateClientOnly(ctx, args)
				must.NoError(t, err)
				must.Eq(t, "client:"+node.ID, args.GetIdentity().String())
				must.NotNil(t, aclObj)
				must.True(t, aclObj.AllowClientOp())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := mock.Node()
			store := testStateStore(t)
			store.UpsertNode(structs.MsgTypeTestSetup, 100, node)
			tc.testFn(t, store, node)
		})
	}
}

func TestResolveACLToken(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func()
	}{
		{
			name: "leader token",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Resolve the token and ensure it's a management token.
				aclResp, err := auth.ResolveToken(auth.getLeaderACL())
				must.NoError(t, err)
				must.NotNil(t, aclResp)
				must.True(t, aclResp.IsManagement())
			},
		},
		{
			name: "anonymous token",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Call the function with an empty input secret ID which is
				// classed as representing anonymous access in clusters with
				// ACLs enabled.
				aclResp, err := auth.ResolveToken("")
				must.NoError(t, err)
				must.NotNil(t, aclResp)
				must.False(t, aclResp.IsManagement())
			},
		},
		{
			name: "anonymous token and acls disabled",
			testFn: func() {
				auth := testDefaultAuthenticator(t)
				auth.aclsEnabled = false

				aclResp, err := auth.ResolveToken("")
				must.NoError(t, err)
				must.NotNil(t, aclResp)
				must.Eq(t, acl.ACLsDisabledACL, aclResp)
				must.True(t, aclResp.IsManagement())
			},
		},
		{
			name: "token not found",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Call the function with randomly generated secret ID which
				// does not exist within state.
				aclResp, err := auth.ResolveToken(uuid.Generate())
				must.ErrorIs(t, err, structs.ErrTokenNotFound)
				must.Nil(t, aclResp)
			},
		},
		{
			name: "token expired",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Create a mock token with an expiration time long in the
				// past, and upsert.
				token := mock.ACLToken()
				token.ExpirationTime = pointer.Of(time.Date(
					1970, time.January, 1, 0, 0, 0, 0, time.UTC))

				err := auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				must.NoError(t, err)

				// Perform the function call which should result in finding the
				// token has expired.
				aclResp, err := auth.ResolveToken(uuid.Generate())
				must.ErrorIs(t, err, structs.ErrTokenNotFound)
				must.Nil(t, aclResp)
			},
		},
		{
			name: "management token",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Generate a management token and upsert this.
				managementToken := mock.ACLToken()
				managementToken.Type = structs.ACLManagementToken
				managementToken.Policies = nil

				err := auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{managementToken})
				must.NoError(t, err)

				// Resolve the token and check that we received a management
				// ACL.
				aclResp, err := auth.ResolveToken(managementToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp)
				must.True(t, aclResp.IsManagement())
				must.Eq(t, acl.ManagementACL, aclResp)
			},
		},
		{
			name: "client token with policies only",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Generate a client token with associated policies and upsert
				// these.
				policy1 := mock.ACLPolicy()
				policy2 := mock.ACLPolicy()
				err := auth.getState().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})

				clientToken := mock.ACLToken()
				clientToken.Policies = []string{policy1.Name, policy2.Name}
				err = auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 20, []*structs.ACLToken{clientToken})
				must.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp)
				must.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs)
				must.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs)
				must.False(t, allowed)

				// Resolve the same token again and ensure we get the same
				// result.
				aclResp2, err := auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp2)
				must.Eq(t, aclResp, aclResp2)

				// Bust the cache by upserting the policy
				err = auth.getState().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 30, []*structs.ACLPolicy{policy1})
				must.Nil(t, err)

				// Resolve the same token again, should get different value
				aclResp3, err := auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp3)
				must.NotEq(t, aclResp2, aclResp3)
			},
		},
		{
			name: "client token with roles only",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Create a client token that only has a link to a role.
				policy1 := mock.ACLPolicy()
				policy2 := mock.ACLPolicy()
				err := auth.getState().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})

				aclRole := mock.ACLRole()
				aclRole.Policies = []*structs.ACLRolePolicyLink{
					{Name: policy1.Name},
					{Name: policy2.Name},
				}
				err = auth.getState().UpsertACLRoles(
					structs.MsgTypeTestSetup, 30, []*structs.ACLRole{aclRole}, false)
				must.NoError(t, err)

				clientToken := mock.ACLToken()
				clientToken.Policies = []string{}
				clientToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRole.ID}}
				err = auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 30, []*structs.ACLToken{clientToken})
				must.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp)
				must.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs)
				must.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs)
				must.False(t, allowed)

				// Remove the policies from the ACL role and ensure the resolution
				// permissions are updated.
				aclRole.Policies = []*structs.ACLRolePolicyLink{}
				err = auth.getState().UpsertACLRoles(
					structs.MsgTypeTestSetup, 40, []*structs.ACLRole{aclRole}, false)
				must.NoError(t, err)

				aclResp, err = auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp)
				must.False(t, aclResp.IsManagement())
				must.False(t, aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))
			},
		},
		{
			name: "client with roles and policies",
			testFn: func() {
				auth := testDefaultAuthenticator(t)

				// Generate two policies, each with a different namespace
				// permission set.
				policy1 := &structs.ACLPolicy{
					Name:        "policy-" + uuid.Generate(),
					Rules:       `namespace "platform" { policy = "write"}`,
					CreateIndex: 10,
					ModifyIndex: 10,
				}
				policy1.SetHash()
				policy2 := &structs.ACLPolicy{
					Name:        "policy-" + uuid.Generate(),
					Rules:       `namespace "web" { policy = "write"}`,
					CreateIndex: 10,
					ModifyIndex: 10,
				}
				policy2.SetHash()

				err := auth.getState().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})
				must.NoError(t, err)

				// Create a role which references the policy that has access to
				// the web namespace.
				aclRole := mock.ACLRole()
				aclRole.Policies = []*structs.ACLRolePolicyLink{{Name: policy2.Name}}
				err = auth.getState().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{aclRole}, false)
				must.NoError(t, err)

				// Create a token which references the policy and role.
				clientToken := mock.ACLToken()
				clientToken.Policies = []string{policy1.Name}
				clientToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRole.ID}}
				err = auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 30, []*structs.ACLToken{clientToken})
				must.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := auth.ResolveToken(clientToken.SecretID)
				must.Nil(t, err)
				must.NotNil(t, aclResp)
				must.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("platform", acl.NamespaceCapabilityListJobs)
				must.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("web", acl.NamespaceCapabilityListJobs)
				must.True(t, allowed)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestIdentityToACLClaim(t *testing.T) {

	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusRunning
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	task := tg.Tasks[0]

	defaultWI := &structs.WorkloadIdentity{Name: "default"}
	claims := structs.NewIdentityClaims(alloc.Job, alloc,
		task.IdentityHandle(defaultWI), task.Identity, time.Now())

	store := testStateStore(t)

	leaderACL := uuid.Generate()

	auth := NewAuthenticator(&AuthenticatorConfig{
		StateFn:        func() *state.StateStore { return store },
		Logger:         testlog.HCLogger(t),
		GetLeaderACLFn: func() string { return leaderACL },
		AclsEnabled:    true,
		VerifyTLS:      true,
		Region:         "global",
		Encrypter:      newTestEncrypter(),
	})

	store.UpsertAllocs(structs.MsgTypeTestSetup, 100,
		[]*structs.Allocation{alloc})

	token, err := auth.encrypter.(*testEncrypter).signClaim(claims)
	must.NoError(t, err)

	ctx := newTestContext(t, "client.nomad.global", "192.168.1.1")
	args := &structs.GenericRequest{}
	args.AuthToken = token

	err = auth.Authenticate(ctx, args)
	must.NoError(t, err)

	claim := IdentityToACLClaim(args.GetIdentity(), auth.getState())
	must.Eq(t, &acl.ACLClaim{
		Namespace: alloc.Job.Namespace,
		Job:       alloc.Job.ID,
		Group:     alloc.TaskGroup,
		Task:      alloc.Job.TaskGroups[0].Tasks[0].Name,
	}, claim)

	must.Nil(t, IdentityToACLClaim(nil, auth.getState()))
}

func TestResolveSecretToken(t *testing.T) {
	ci.Parallel(t)
	auth := testDefaultAuthenticator(t)

	testCases := []struct {
		name   string
		testFn func()
	}{
		{
			name: "valid token",
			testFn: func() {

				// Generate and upsert a token.
				token := mock.ACLToken()
				err := auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				must.NoError(t, err)

				// Attempt to look up the token and perform checks.
				tokenResp, err := auth.resolveSecretToken(token.SecretID)
				must.NoError(t, err)
				must.NotNil(t, tokenResp)
				must.Eq(t, token, tokenResp)
			},
		},
		{
			name: "anonymous token",
			testFn: func() {

				// Call the function with an empty input secret ID which is
				// classed as representing anonymous access in clusters with
				// ACLs enabled.
				tokenResp, err := auth.resolveSecretToken("")
				must.NoError(t, err)
				must.NotNil(t, tokenResp)
				must.Eq(t, structs.AnonymousACLToken, tokenResp)
			},
		},
		{
			name: "token not found",
			testFn: func() {

				// Call the function with randomly generated secret ID which
				// does not exist within state.
				tokenResp, err := auth.resolveSecretToken(uuid.Generate())
				must.ErrorIs(t, err, structs.ErrTokenNotFound)
				must.Nil(t, tokenResp)
			},
		},
		{
			name: "token expired",
			testFn: func() {

				// Create a mock token with an expiration time long in the
				// past, and upsert.
				token := mock.ACLToken()
				token.ExpirationTime = pointer.Of(time.Date(
					1970, time.January, 1, 0, 0, 0, 0, time.UTC))

				err := auth.getState().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				must.NoError(t, err)

				// Perform the function call which should result in finding the
				// token has expired.
				tokenResp, err := auth.resolveSecretToken(uuid.Generate())
				must.ErrorIs(t, err, structs.ErrTokenNotFound)
				must.Nil(t, tokenResp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestResolveClaims(t *testing.T) {
	ci.Parallel(t)

	auth := testDefaultAuthenticator(t)
	index := uint64(100)

	alloc := mock.Alloc()
	dispatchAlloc := mock.Alloc()
	dispatchAlloc.Job.ParentID = alloc.JobID

	claims := &structs.IdentityClaims{
		Namespace:    alloc.Namespace,
		JobID:        alloc.Job.ID,
		AllocationID: alloc.ID,
		TaskName:     alloc.Job.TaskGroups[0].Tasks[0].Name,
	}

	dispatchClaims := &structs.IdentityClaims{
		Namespace:    dispatchAlloc.Namespace,
		JobID:        dispatchAlloc.Job.ID,
		AllocationID: dispatchAlloc.ID,
		TaskName:     dispatchAlloc.Job.TaskGroups[0].Tasks[0].Name,
	}

	// unrelated policy
	policy0 := mock.ACLPolicy()

	// policy for job
	policy1 := mock.ACLPolicy()
	policy1.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
	}

	// policy for job and group
	policy2 := mock.ACLPolicy()
	policy2.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
	}

	// policy for job and group	and task
	policy3 := mock.ACLPolicy()
	policy3.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
		Task:      claims.TaskName,
	}

	// policy for job and group	but different task
	policy4 := mock.ACLPolicy()
	policy4.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
		Task:      "another",
	}

	// policy for job but different group
	policy5 := mock.ACLPolicy()
	policy5.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     "another",
	}

	// policy for same namespace but different job
	policy6 := mock.ACLPolicy()
	policy6.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     "another",
	}

	// policy for same job in different namespace
	policy7 := mock.ACLPolicy()
	policy7.JobACL = &structs.JobACL{
		Namespace: "another",
		JobID:     claims.JobID,
	}

	aclObj, err := auth.resolveClaims(claims)
	must.Nil(t, aclObj)
	must.EqError(t, err, "allocation does not exist")

	// upsert the allocation
	index++
	err = auth.getState().UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc, dispatchAlloc})
	must.NoError(t, err)

	// Resolve claims and check we that the ACL object without policies provides no access
	aclObj, err = auth.resolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.False(t, aclObj.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))

	// Add the policies
	index++
	err = auth.getState().UpsertACLPolicies(structs.MsgTypeTestSetup, index, []*structs.ACLPolicy{
		policy0, policy1, policy2, policy3, policy4, policy5, policy6, policy7})
	must.NoError(t, err)

	// Re-resolve and check that the resulting ACL looks reasonable
	aclObj, err = auth.resolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.False(t, aclObj.IsManagement())
	must.True(t, aclObj.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))
	must.False(t, aclObj.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs))

	// Resolve the same claim again, should get cache value
	aclObj2, err := auth.resolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.Eq(t, aclObj, aclObj2, must.Sprintf("expected cached value"))

	policies, err := auth.ResolvePoliciesForClaims(claims)
	must.NoError(t, err)
	must.Len(t, 3, policies)
	must.SliceContainsAll(t, policies, []*structs.ACLPolicy{policy1, policy2, policy3})

	// Check the dispatch claims
	aclObj3, err := auth.resolveClaims(dispatchClaims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.Eq(t, aclObj, aclObj3, must.Sprintf("expected cached value"))

	dispatchPolicies, err := auth.ResolvePoliciesForClaims(dispatchClaims)
	must.NoError(t, err)
	must.Len(t, 3, dispatchPolicies)
	must.SliceContainsAll(t, dispatchPolicies, []*structs.ACLPolicy{policy1, policy2, policy3})

}

func testStateStore(t *testing.T) *state.StateStore {
	sconfig := &state.StateStoreConfig{
		Logger:             testlog.HCLogger(t),
		Region:             "global",
		JobTrackedVersions: structs.JobDefaultTrackedVersions,
	}
	store, err := state.NewStateStore(sconfig)
	must.NoError(t, err)
	return store
}

func testDefaultAuthenticator(t *testing.T) *Authenticator {
	leaderACL := uuid.Generate()
	store := testStateStore(t)
	return NewAuthenticator(&AuthenticatorConfig{
		StateFn:        func() *state.StateStore { return store },
		Logger:         testlog.HCLogger(t),
		GetLeaderACLFn: func() string { return leaderACL },
		AclsEnabled:    true,
		VerifyTLS:      true,
		Region:         "global",
		Encrypter:      nil,
	})
}

type testContext struct {
	isTLS    bool
	cert     *x509.Certificate
	remoteIP net.IP
}

const noTLSCtx = ""

func newTestContext(t *testing.T, tlsName, ipAddr string) *testContext {
	t.Helper()
	ip := net.ParseIP(ipAddr)
	must.NotNil(t, ip, must.Sprintf("could not parse ipAddr=%s", ipAddr))
	ctx := &testContext{
		remoteIP: ip,
	}
	if tlsName != "" {
		ctx.isTLS = true
		ctx.cert = &x509.Certificate{
			Subject: pkix.Name{
				CommonName: tlsName,
			},
		}
	}
	return ctx
}

func (ctx *testContext) GetRemoteIP() (net.IP, error) {
	if ctx == nil {
		return nil, nil
	}
	if len(ctx.remoteIP) == 0 {
		return nil, errors.New("could not determine remote IP from context")
	}
	return ctx.remoteIP, nil
}

func (ctx *testContext) IsTLS() bool {
	if ctx == nil {
		return false
	}
	return ctx.isTLS
}

func (ctx *testContext) IsStatic() bool {
	return ctx == nil
}

func (ctx *testContext) Certificate() *x509.Certificate {
	if ctx == nil {
		return nil
	}
	return ctx.cert
}

type testEncrypter struct {
	key *rsa.PrivateKey
}

func newTestEncrypter() *testEncrypter {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return &testEncrypter{
		key: k,
	}
}

func (te *testEncrypter) signClaim(claims *structs.IdentityClaims) (string, error) {

	opts := (&jose.SignerOptions{}).WithType("JWT")
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: te.key}, opts)
	if err != nil {
		return "", err
	}
	raw, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (te *testEncrypter) VerifyClaim(tokenString string) (*structs.IdentityClaims, error) {

	token, err := jwt.ParseSigned(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed token: %w", err)
	}
	pubKey := te.key.Public()

	claims := &structs.IdentityClaims{}
	if err := token.Claims(pubKey, claims); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}
	expect := jwt.Expected{}
	if err := claims.Validate(expect); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return claims, nil
}

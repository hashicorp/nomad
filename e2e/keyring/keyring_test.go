// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package keyring

import (
	"encoding/json"
	"testing"

	"github.com/go-jose/go-jose/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/shoenig/test/must"
)

func TestKeyringRotation(t *testing.T) {

	nc := e2eutil.NomadClient(t)

	currentKeys, activeKeyID := getKeyMeta(t, nc)
	must.NotEq(t, "", activeKeyID, must.Sprint("expected an active key"))

	keyset := getJWKS(t)

	must.Len(t, len(currentKeys), keyset.Keys)
	for _, key := range keyset.Keys {
		must.MapContainsKey(t, currentKeys, key.KeyID)
	}

	out, err := e2eutil.Commandf("nomad operator root keyring rotate -verbose -prepublish 1h")
	must.NoError(t, err)
	cols, err := e2eutil.ParseColumns(out)
	must.NoError(t, err)
	must.Greater(t, 0, len(cols))
	newKeyID := cols[0]["Key"]
	must.Eq(t, "prepublished", cols[0]["State"], must.Sprint("expected new key to be prepublished"))

	newCurrentKeys, newActiveKeyID := getKeyMeta(t, nc)
	must.NotEq(t, "", newActiveKeyID, must.Sprint("expected an active key"))
	must.Eq(t, activeKeyID, newActiveKeyID, must.Sprint("active key should not have rotated yet"))
	must.Greater(t, len(currentKeys), len(newCurrentKeys), must.Sprint("expected more keys after prepublishing"))

	keyset = getJWKS(t)
	must.Len(t, len(newCurrentKeys), keyset.Keys, must.Sprint("number of keys in jwks keyset should match keyring"))
	for _, key := range keyset.Keys {
		must.MapContainsKey(t, newCurrentKeys, key.KeyID, must.Sprint("jwks keyset contains unexpected key"))
	}
	must.SliceContainsFunc(t, keyset.Keys, newKeyID, func(a jose.JSONWebKey, b string) bool {
		return a.KeyID == b
	}, must.Sprint("expected prepublished key to appear in JWKS endpoint"))
}

func getKeyMeta(t *testing.T, nc *api.Client) (map[string]*api.RootKeyMeta, string) {
	t.Helper()
	keyMetas, _, err := nc.Keyring().List(nil)
	must.NoError(t, err)

	currentKeys := map[string]*api.RootKeyMeta{}
	var activeKeyID string
	for _, keyMeta := range keyMetas {
		currentKeys[keyMeta.KeyID] = keyMeta
		if keyMeta.State == api.RootKeyStateActive {
			activeKeyID = keyMeta.KeyID
		}
	}
	must.NotEq(t, "", activeKeyID, must.Sprint("expected an active key"))
	return currentKeys, activeKeyID
}

func getJWKS(t *testing.T) *jose.JSONWebKeySet {
	t.Helper()
	out, err := e2eutil.Commandf("nomad operator api /.well-known/jwks.json")
	must.NoError(t, err)

	keyset := &jose.JSONWebKeySet{}
	err = json.Unmarshal([]byte(out), keyset)
	must.NoError(t, err)

	return keyset
}

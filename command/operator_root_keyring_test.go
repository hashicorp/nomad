// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestOperatorRootKeyringRemoveCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorRootKeyringRemoveCommand{}
}

func TestOperatorRootKeyringRemoveCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, addr := testServer(t, false, nil)
	defer srv.Shutdown()
	store := srv.Agent.Server().State()

	// Insert two inactive keys whose IDs share an 8-char prefix
	longID := uuid.Generate()
	longID2 := longID[0:8] + uuid.Generate()[8:]

	for i, id := range []string{longID, longID2} {
		meta := structs.NewRootKeyMeta()
		meta.KeyID = id
		key := structs.NewRootKey(meta).MakeInactive()
		must.NoError(t, store.UpsertRootKey(uint64(1000+i), key, false))
	}

	ui := cli.NewMockUi()
	c := &OperatorRootKeyringRemoveCommand{Meta: Meta{Ui: ui}}

	// An empty ID is rejected before any lookup
	code := c.Run([]string{"-address=" + addr, ""})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "This command requires one argument")
	ui.ErrorWriter.Reset()

	// An ambiguous prefix must not remove anything
	code = c.Run([]string{"-address=" + addr, longID[0:8]})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Prefix matched multiple keys")
	must.StrContains(t, ui.ErrorWriter.String(), longID)
	must.StrContains(t, ui.ErrorWriter.String(), longID2)
	keys, _, err := client.Keyring().List(nil)
	must.NoError(t, err)
	must.Len(t, 3, keys)
	ui.ErrorWriter.Reset()

	// A non-hex prefix can never match a key ID
	code = c.Run([]string{"-address=" + addr, "zzzzzzzz"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "No encryption key(s)")
	ui.ErrorWriter.Reset()

	// A UUID-shaped argument is passed through without resolution
	code = c.Run([]string{"-address=" + addr, longID2})
	must.Zero(t, code, must.Sprintf("expected exit 0, got err: %s", ui.ErrorWriter.String()))
	ui.OutputWriter.Reset()

	// The abbreviated ID, now unique, resolves to the full key regardless
	// of case
	code = c.Run([]string{"-address=" + addr, strings.ToUpper(longID[0:8])})
	must.Zero(t, code, must.Sprintf("expected exit 0, got err: %s", ui.ErrorWriter.String()))
	must.StrContains(t, ui.OutputWriter.String(), longID)

	keys, _, err = client.Keyring().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, keys)
	must.NotEq(t, longID, keys[0].KeyID)
	must.NotEq(t, longID2, keys[0].KeyID)
}

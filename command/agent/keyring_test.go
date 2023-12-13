// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
)

func TestAgent_LoadKeyrings(t *testing.T) {
	ci.Parallel(t)
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	agent1 := NewTestAgent(t, t.Name(), nil)
	defer agent1.Shutdown()

	c := agent1.server.GetConfig()
	if c.SerfConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfConfig.KeyringFile)
	}
	if c.SerfConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}

	// Server should auto-load WAN keyring files
	agent2 := &TestAgent{
		T:      t,
		Name:   t.Name() + "2",
		Key:    key,
		logger: testlog.HCLogger(t),
	}
	agent2.Start()
	defer agent2.Shutdown()

	c = agent2.server.GetConfig()
	if c.SerfConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c.SerfConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}
}

func TestAgent_InitKeyring(t *testing.T) {
	ci.Parallel(t)
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	expected := fmt.Sprintf(`["%s"]`, key1)

	dir := t.TempDir()

	file := filepath.Join(dir, "keyring")

	logger := hclog.NewNullLogger()

	// First initialize the keyring
	if err := initKeyring(file, key1, logger); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}

	// Try initializing again with a different key
	if err := initKeyring(file, key2, logger); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Content should still be the same
	content, err = os.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}
}

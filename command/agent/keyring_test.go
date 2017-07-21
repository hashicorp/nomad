package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAgent_LoadKeyrings(t *testing.T) {
	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	agent1 := NewTestAgent(t.Name(), nil)
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
		Name: t.Name() + "2",
		Key:  key,
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
	t.Parallel()
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	expected := fmt.Sprintf(`["%s"]`, key1)

	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "keyring")

	// First initialize the keyring
	if err := initKeyring(file, key1); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}

	// Try initializing again with a different key
	if err := initKeyring(file, key2); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Content should still be the same
	content, err = ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}
}

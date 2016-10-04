package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAgent_LoadKeyrings(t *testing.T) {
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	dir1, agent1 := makeAgent(t, nil)
	defer os.RemoveAll(dir1)
	defer agent1.Shutdown()

	c := agent1.server.GetConfig()
	if c.SerfConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfConfig.KeyringFile)
	}
	if c.SerfConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}

	// Server should auto-load LAN and WAN keyring files
	dir2, agent2 := makeAgent(t, func(c *Config) {
		file := filepath.Join(c.DataDir, serfKeyring)
		if err := initKeyring(file, key); err != nil {
			t.Fatalf("err: %s", err)
		}
	})
	defer os.RemoveAll(dir2)
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
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	expected := fmt.Sprintf(`["%s"]`, key1)

	dir, err := ioutil.TempDir("", "consul")
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

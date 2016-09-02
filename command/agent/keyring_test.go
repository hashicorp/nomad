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

	c := agent1.config.NomadConfig

	if c.SerfConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfConfig.KeyringFile)
	}
	if c.SerfConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}

	// Server should auto-load WAN keyring file
	dir2, agent2 := makeAgentKeyring(t, key)
	defer os.RemoveAll(dir2)
	defer agent2.Shutdown()

	c = agent2.config.NomadConfig

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

func TestAgent_ListKeys(t *testing.T) {
	key := "tbLJg26ZJyJ9pK3qhc9jig=="
	dir, agent := makeAgentKeyring(t, key)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	responses, err := agent.ListKeys()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(responses.Responses) != 1 {
		t.Fatal("err: should have only 1 response")
	}

	keys := responses.Responses[0].Keys
	if len(keys) != 1 {
		t.Fatal("err: should have only 1 key")
	}

	if _, ok := keys[key]; !ok {
		t.Fatal("err: Key not found %s", key)
	}
}

func TestAgent_InstallKey(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	dir, agent := makeAgentKeyring(t, key1)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	responses, err := agent.InstallKey(key2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	responses, err = agent.ListKeys()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	keys := responses.Responses[0].Keys
	if len(keys) != 2 {
		t.Fatal("err: should have 2 keys")
	}
	if _, ok := keys[key2]; !ok {
		t.Fatal("err: Key not found %s", key2)
	}
}

func TestAgent_UseKey(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	dir, agent := makeAgentKeyring(t, key1)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	_, err := agent.InstallKey(key2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	_, err = agent.UseKey(key2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestAgent_RemoveKey(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	dir, agent := makeAgentKeyring(t, key1)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	_, err := agent.InstallKey(key2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	responses, err := agent.RemoveKey(key2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	responses, err = agent.ListKeys()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	keys := responses.Responses[0].Keys
	if len(keys) != 1 {
		t.Fatal("err: should have 1 key")
	}
	if _, ok := keys[key1]; !ok {
		t.Fatal("err: Key not found %s", key1)
	}
}

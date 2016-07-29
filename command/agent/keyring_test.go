package agent

import (
  "os"
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
	dir2, agent2 := makeAgentKeyring(t, nil, key)
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

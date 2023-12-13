// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const (
	serfKeyring = "server/serf.keyring"
)

// initKeyring will create a keyring file at a given path.
func initKeyring(path, key string, l log.Logger) error {
	var keys []string

	if keyBytes, err := base64.StdEncoding.DecodeString(key); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	} else if err := memberlist.ValidateKey(keyBytes); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	}

	// Check for AES-256 key size (32-bytes)
	if len(key) < 32 {
		var encMethod string
		switch len(key) {
		case 16:
			encMethod = "AES-128"
		case 24:
			encMethod = "AES-192"
		}
		msg := fmt.Sprintf("given %d-byte gossip key enables %s encryption, generate a 32-byte key to enable AES-256", len(key), encMethod)
		l.Info(msg)
	}

	// Just exit if the file already exists.
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	keys = append(keys, key)
	keyringBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := fh.Write(keyringBytes); err != nil {
		os.Remove(path)
		return err
	}

	return nil
}

// loadKeyringFile will load a gossip encryption keyring out of a file. The file
// must be in JSON format and contain a list of encryption key strings.
func loadKeyringFile(c *serf.Config) error {
	if c.KeyringFile == "" {
		return nil
	}

	if _, err := os.Stat(c.KeyringFile); err != nil {
		return err
	}

	// Read in the keyring file data
	keyringData, err := os.ReadFile(c.KeyringFile)
	if err != nil {
		return err
	}

	// Decode keyring JSON
	keys := make([]string, 0)
	if err := json.Unmarshal(keyringData, &keys); err != nil {
		return err
	}

	// Decode base64 values
	keysDecoded := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return err
		}
		keysDecoded[i] = keyBytes
	}

	// Guard against empty keyring
	if len(keysDecoded) == 0 {
		return fmt.Errorf("no keys present in keyring file: %s", c.KeyringFile)
	}

	// Create the keyring
	keyring, err := memberlist.NewKeyring(keysDecoded, keysDecoded[0])
	if err != nil {
		return err
	}

	c.MemberlistConfig.Keyring = keyring

	// Success!
	return nil
}

package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBridgeFingerprint_checkKMod(t *testing.T) {
	require := require.New(t)
	f := &BridgeFingerprint{}
	require.NoError(f.checkKMod("hid"))
	require.Error(f.checkKMod("nonexistentmodule"))
}
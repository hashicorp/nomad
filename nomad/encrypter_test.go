package nomad

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TestEncrypter_LoadSave exercises round-tripping keys to disk
func TestEncrypter_LoadSave(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	encrypter := NewEncrypter(tmpDir)

	algos := []structs.EncryptionAlgorithm{
		structs.EncryptionAlgorithmAES256GCM,
		structs.EncryptionAlgorithmXChaCha20,
	}

	for _, algo := range algos {
		t.Run(string(algo), func(t *testing.T) {
			key, err := structs.NewRootKey(algo)
			require.NoError(t, err)
			require.NoError(t, encrypter.SaveKeyToStore(key))

			gotKey, err := encrypter.LoadKeyFromStore(
				filepath.Join(tmpDir, key.Meta.KeyID+".json"))
			require.NoError(t, err)
			require.Len(t, gotKey.Key, 32)
		})
	}
}

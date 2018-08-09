package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// mockKVStore tracks puts and is useful for testing KVCodec's write-on-change
// code.
type mockKVStore struct {
	puts int
}

func (mockKVStore) Get(key []byte) (val []byte) {
	return nil
}

func (m *mockKVStore) Put(key, val []byte) error {
	m.puts++
	return nil
}

func (mockKVStore) Writable() bool {
	return true
}

// TestKVCodec_PutHash asserts that Puts on the underlying kvstore only occur
// when the data actually changes.
func TestKVCodec_PutHash(t *testing.T) {
	require := require.New(t)
	codec := newKeyValueCodec()

	// Create arguments for Put
	kv := new(mockKVStore)
	path := "path-path"
	key := []byte("key1")
	val := &struct {
		Val int
	}{
		Val: 1,
	}

	// Initial Put should be written
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(1, kv.puts)

	// Writing the same values again should be a noop
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(1, kv.puts)

	// Changing the value should write again
	val.Val++
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(2, kv.puts)

	// Changing the key should write again
	key = []byte("key2")
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(3, kv.puts)

	// Changing the path should write again
	path = "new-path"
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(4, kv.puts)

	// Writing the same values again should be a noop
	require.NoError(codec.Put(kv, path, key, val))
	require.Equal(4, kv.puts)
}

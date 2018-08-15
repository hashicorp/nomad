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

func (mockKVStore) Path() []byte {
	return []byte{}
}

func (m *mockKVStore) Bucket(name []byte) kvStore {
	return m
}

func (m *mockKVStore) CreateBucket(key []byte) (kvStore, error) {
	return m, nil
}

func (m *mockKVStore) CreateBucketIfNotExists(key []byte) (kvStore, error) {
	return m, nil
}

func (m *mockKVStore) DeleteBucket(key []byte) error {
	return nil
}

func (mockKVStore) Get(key []byte) (val []byte) {
	return nil
}

func (m *mockKVStore) Put(key, val []byte) error {
	m.puts++
	return nil
}

// TestKVCodec_PutHash asserts that Puts on the underlying kvstore only occur
// when the data actually changes.
func TestKVCodec_PutHash(t *testing.T) {
	require := require.New(t)
	codec := newKeyValueCodec()

	// Create arguments for Put
	kv := new(mockKVStore)
	key := []byte("key1")
	val := &struct {
		Val int
	}{
		Val: 1,
	}

	// Initial Put should be written
	require.NoError(codec.Put(kv, key, val))
	require.Equal(1, kv.puts)

	// Writing the same values again should be a noop
	require.NoError(codec.Put(kv, key, val))
	require.Equal(1, kv.puts)

	// Changing the value should write again
	val.Val++
	require.NoError(codec.Put(kv, key, val))
	require.Equal(2, kv.puts)

	// Changing the key should write again
	key = []byte("key2")
	require.NoError(codec.Put(kv, key, val))
	require.Equal(3, kv.puts)

	// Writing the same values again should be a noop
	require.NoError(codec.Put(kv, key, val))
	require.Equal(3, kv.puts)
}

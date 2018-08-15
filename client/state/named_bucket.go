package state

import "github.com/boltdb/bolt"

// namedBucket is a wrapper around bolt.Bucket's to preserve their path
// information and expose it via the Path() method.
//
// Knowing the full bucket path to a key is necessary for tracking accesses in
// another datastructure such as the hashing writer keyValueCodec.
type namedBucket struct {
	path []byte
	name []byte
	bkt  *bolt.Bucket
}

// newNamedBucket from a bolt transaction.
func newNamedBucket(tx *bolt.Tx, root []byte) *namedBucket {
	b := tx.Bucket(root)
	if b == nil {
		return nil
	}

	return &namedBucket{
		path: root,
		name: root,
		bkt:  b,
	}
}

// createNamedBucketIfNotExists from a bolt transaction.
func createNamedBucketIfNotExists(tx *bolt.Tx, root []byte) (*namedBucket, error) {
	b, err := tx.CreateBucketIfNotExists(root)
	if err != nil {
		return nil, err
	}

	return &namedBucket{
		path: root,
		name: root,
		bkt:  b,
	}, nil
}

// Path to this bucket (including this bucket).
func (n *namedBucket) Path() []byte {
	return n.path
}

// Name of this bucket.
func (n *namedBucket) Name() []byte {
	return n.name
}

// Bucket returns a bucket inside the current one or nil if the bucket does not
// exist.
func (n *namedBucket) Bucket(name []byte) kvStore {
	b := n.bkt.Bucket(name)
	if b == nil {
		return nil
	}

	return &namedBucket{
		path: n.chBkt(name),
		name: name,
		bkt:  b,
	}
}

// CreateBucketIfNotExists creates a bucket if it doesn't exist and returns it
// or an error.
func (n *namedBucket) CreateBucketIfNotExists(name []byte) (kvStore, error) {
	b, err := n.bkt.CreateBucketIfNotExists(name)
	if err != nil {
		return nil, err
	}

	return &namedBucket{
		path: n.chBkt(name),
		name: name,
		bkt:  b,
	}, nil
}

// CreateBucket creates a bucket and returns it.
func (n *namedBucket) CreateBucket(name []byte) (kvStore, error) {
	b, err := n.bkt.CreateBucket(name)
	if err != nil {
		return nil, err
	}

	return &namedBucket{
		path: n.chBkt(name),
		name: name,
		bkt:  b,
	}, nil
}

// DeleteBucket calls DeleteBucket on the underlying bolt.Bucket.
func (n *namedBucket) DeleteBucket(name []byte) error {
	return n.bkt.DeleteBucket(name)
}

// Get calls Get on the underlying bolt.Bucket.
func (n *namedBucket) Get(key []byte) []byte {
	return n.bkt.Get(key)
}

// Put calls Put on the underlying bolt.Bucket.
func (n *namedBucket) Put(key, value []byte) error {
	return n.bkt.Put(key, value)
}

// chBkt is like chdir but for buckets: it appends the new name to the end of
// a copy of the path and returns it.
func (n *namedBucket) chBkt(name []byte) []byte {
	// existing path + new path element + path separator
	path := make([]byte, len(n.path)+len(name)+1)
	copy(path[0:len(n.path)], n.path)
	path[len(n.path)] = '/'
	copy(path[len(n.path)+1:], name)

	return path
}

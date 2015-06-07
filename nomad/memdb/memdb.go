package memdb

import (
	"sync"

	"github.com/hashicorp/go-immutable-radix"
)

// MemDB is an in-memory database. It provides a table abstraction,
// which is used to store objects (rows) with multiple indexes based
// on values. The database makes use of immutable radix trees to provide
// transactions and MVCC.
type MemDB struct {
	schema *DBSchema
	root   *iradix.Tree

	// There can only be a single writter at once
	writer sync.Mutex
}

// NewMemDB creates a new MemDB with the given schema
func NewMemDB(schema *DBSchema) (*MemDB, error) {
	// Validate the schema
	if err := schema.Validate(); err != nil {
		return nil, err
	}

	// Create the MemDB
	db := &MemDB{
		schema: schema,
		root:   iradix.New(),
	}
	return db, nil
}

// Txn is used to start a new transaction, in either read or write mode.
// There can only be a single concurrent writer, but any number of readers.
func (db *MemDB) Txn(write bool) *Txn {
	txn := &Txn{
		db:    db,
		write: write,
		root:  db.root,
	}
	if write {
		txn.rootTxn = txn.root.Txn()
	}
	return txn
}

package memdb

import (
	"fmt"

	"github.com/hashicorp/go-immutable-radix"
)

// Txn is a transaction against a MemDB. This can be a read or write transaction.
type Txn struct {
	db      *MemDB
	write   bool
	root    *iradix.Tree
	rootTxn *iradix.Txn
}

// Abort is used to cancel this transaction. This is a noop for read transactions.
func (txn *Txn) Abort() {
	// Noop for a read transaction
	if !txn.write {
		return
	}

	// Check if already aborted or committed
	if txn.root == nil {
		return
	}

	// Release the writer lock since this is invalid
	txn.db.writer.Unlock()
	txn.root = nil
	txn.rootTxn = nil
}

// Commit is used to finalize this transaction. This is a noop for read transactions.
func (txn *Txn) Commit() {
	// Noop for a read transaction
	if !txn.write {
		return
	}

	// Check if already aborted or committed
	if txn.root == nil {
		return
	}

	// Update the root of the DB
	txn.db.root = txn.rootTxn.Commit()

	// Clear the txn
	txn.root = nil
	txn.rootTxn = nil

	// Release the writer lock since this is invalid
	txn.db.writer.Unlock()
}

// Insert is used to add or update an object into the given table
func (txn *Txn) Insert(table string, obj interface{}) error {
	if !txn.write {
		return fmt.Errorf("cannot insert in read-only transaction")
	}
	return nil
}

func (txn *Txn) Delete(table, index string, args ...interface{}) error {
	if !txn.write {
		return fmt.Errorf("cannot delete in read-only transaction")
	}
	return nil
}

type ResultIterator interface {
	Next() interface{}
}

func (txn *Txn) Get(table, index string, args ...interface{}) (ResultIterator, error) {
	return nil, nil
}

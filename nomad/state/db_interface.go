package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Txn is a transaction against a state store, wrapping a memdb.Txn.
// This can be a read or write transaction.
type Txn interface {
	// Methods not in memdb.Txn
	Index() uint64
	Commit() error // note: this signature is different from memdb.Commit()

	// Methods of memdb.Txn
	Abort()
	Changes() memdb.Changes
	Defer(fn func())
	Delete(table string, obj any) error
	DeleteAll(table, index string, args ...any) (int, error)
	DeletePrefix(table string, prefix_index string, prefix string) (bool, error)
	First(table, index string, args ...any) (any, error)
	FirstWatch(table, index string, args ...any) (<-chan struct{}, any, error)
	Get(table, index string, args ...any) (memdb.ResultIterator, error)
	GetReverse(table, index string, args ...any) (memdb.ResultIterator, error)
	Insert(table string, obj any) error
	TrackChanges()

	// Not implemented methods of memdb.Txn
	// Commit()
	// Last(table, index string, args ...any) (any, error)
	// LastWatch(table, index string, args ...any) (<-chan struct{}, any, error)
	// LongestPrefix(table, index string, args ...any) (any, error)
	// LowerBound(table, index string, args ...any) (memdb.ResultIterator, error)
	// ReverseLowerBound(table, index string, args ...any) (memdb.ResultIterator, error)
	// Snapshot() *memdb.Txn

}

// ReadTxn is implemented by memdb.Txn to perform read operations.
type ReadTxn interface {
	Get(table, index string, args ...interface{}) (memdb.ResultIterator, error)
	First(table, index string, args ...interface{}) (interface{}, error)
	FirstWatch(table, index string, args ...interface{}) (<-chan struct{}, interface{}, error)
	Abort()
}

// MemDBWrapper wraps a memdb.MemDB so that we can return Txn interfaces instead
// of memdb.Txn pointers
type MemDBWrapper interface {
	ReadTxn() Txn
	WriteTxn(uint64) Txn
	WriteTxnMsgT(structs.MessageType, uint64) Txn
	WriteTxnRestore() Txn
	Snapshot() *memdb.MemDB
	Publisher() *stream.EventBroker
}

// baseMemDBWrapper is a thin wrapper around memdb.DB used as the innermost
// MemDBWrapper
type baseMemDBWrapper struct {
	memdb *memdb.MemDB
}

func NewBaseMemDBWrapper(db *memdb.MemDB) *baseMemDBWrapper {
	return &baseMemDBWrapper{memdb: db}
}

func (b *baseMemDBWrapper) ReadTxn() Txn {
	return &baseTxn{Txn: b.memdb.Txn(false)}
}

func (b *baseMemDBWrapper) WriteTxn(index uint64) Txn {
	return &baseTxn{Txn: b.memdb.Txn(true), index: index}
}

func (b *baseMemDBWrapper) WriteTxnMsgT(_ structs.MessageType, index uint64) Txn {
	return &baseTxn{Txn: b.memdb.Txn(true), index: index}
}

func (b *baseMemDBWrapper) WriteTxnRestore() Txn {
	return &baseTxn{Txn: b.memdb.Txn(true), index: 0}
}

func (c *baseMemDBWrapper) Snapshot() *memdb.MemDB {
	return c.memdb.Snapshot()
}

func (c *baseMemDBWrapper) Publisher() *stream.EventBroker {
	return nil
}

type baseTxn struct {
	*memdb.Txn
	index uint64
}

func (tx *baseTxn) Commit() error {
	tx.Txn.Commit()
	return nil
}

func (tx *baseTxn) Index() uint64 {
	return tx.index
}

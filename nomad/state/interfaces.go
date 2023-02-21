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
	MsgType() structs.MessageType
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

	// Unimplemented methods of memdb.Txn because we don't call them (directly)
	// from wrappers
	//
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
// MemDBWrapper; it translates between MemDBWrapper methods and memdb.DB
// methods. All other MemDBWrappers should wrap this one so that they can call
// their inner MemDBWrapper methods directly.
type baseMemDBWrapper struct {
	memdb *memdb.MemDB
}

func NewBaseMemDBWrapper(db *memdb.MemDB) *baseMemDBWrapper {
	return &baseMemDBWrapper{memdb: db}
}

// ReadTxn ... TODO
func (b *baseMemDBWrapper) ReadTxn() Txn {
	return &baseTxn{Txn: b.memdb.Txn(false)}
}

// WriteTxn returns a Txn wrapping a memdb.Txn suitable for writes to the state store.
//
// The idx argument must be the index of the current Raft operation. Most
// mutations to state should happen as part of a raft apply, so the index of the
// log being applied should be the one passed to WriteTxn. The only exception
// are transactions executed on empty memdb.DB as part of Restore, which should
// use WriteTxnRestore instead.
func (b *baseMemDBWrapper) WriteTxn(idx uint64) Txn {
	return &baseTxn{
		// Note: the zero value of structs.MessageType is noderegistration.
		msgType: structs.IgnoreUnknownTypeFlag,
		index:   idx,
		Txn:     b.memdb.Txn(true),
	}
}

// WriteTxnMsgT returns a Txn wrapping a memdb.Txn suitable for writes to the
// state store. This is the same as WriteTxn but includes the MessageType, which
// is useful for wrappers that change behavior based on that value.
func (b *baseMemDBWrapper) WriteTxnMsgT(msgType structs.MessageType, idx uint64) Txn {
	return &baseTxn{
		msgType: msgType,
		index:   idx,
		Txn:     b.memdb.Txn(true),
	}
}

// WriteTxnRestore returns a Txn wrapping a memdb.Txn suitable for writes. This
// should only be used in Restore where we need to replace the entire contents
// of the store, and wrappers may need to change their behavior based on
// that. WriteTxnRestore uses a zero index since the whole restore doesn't
// really occur at one index - the effect is to write many values that were
// previously written across many indexes.
func (b *baseMemDBWrapper) WriteTxnRestore() Txn {
	return &baseTxn{Txn: b.memdb.Txn(true), index: 0}
}

// Snapshot ... TODO
func (c *baseMemDBWrapper) Snapshot() *memdb.MemDB {
	return c.memdb.Snapshot()
}

// Publisher ... TODO
func (c *baseMemDBWrapper) Publisher() *stream.EventBroker {
	return nil
}

// baseTxn is the Txn returned by baseMemDBWrapper methods. Note that the inner
// transaction for a baseTxn is a real memdb.Txn, unlike the other wrappers
// which wrap other state.Txn interfaces
type baseTxn struct {
	msgType structs.MessageType
	index   uint64
	*memdb.Txn
}

// MsgType returns a MessageType from the Txn's context. If the context is empty
// or the value isn't set IgnoreUnknownTypeFlag will be returned to signal that
// the MsgType is unknown.
func (tx *baseTxn) MsgType() structs.MessageType {
	return tx.msgType
}

// Index returns the Index of the Txn. This will be 0 if the Txn is part of a
// restore.
func (tx *baseTxn) Index() uint64 {
	return tx.index
}

// Commit commits the inner memdb transaction
func (tx *baseTxn) Commit() error {
	tx.Txn.Commit()
	return nil
}

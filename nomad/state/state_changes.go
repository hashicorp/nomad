// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ReadTxn is implemented by memdb.Txn to perform read operations.
type ReadTxn interface {
	Get(table, index string, args ...interface{}) (memdb.ResultIterator, error)
	First(table, index string, args ...interface{}) (interface{}, error)
	FirstWatch(table, index string, args ...interface{}) (<-chan struct{}, interface{}, error)
	Abort()
}

// Changes wraps a memdb.Changes to include the index at which these changes
// were made.
type Changes struct {
	// Index is the latest index at the time these changes were committed.
	Index   uint64
	Changes memdb.Changes
	MsgType structs.MessageType
}

// changeTrackerDB is a thin wrapper around memdb.DB which enables TrackChanges on
// all write transactions. When the transaction is committed the changes are
// sent to the EventBroker which will create and emit change events.
type changeTrackerDB struct {
	memdb          *memdb.MemDB
	publisher      *stream.EventBroker
	processChanges changeProcessor
}

func NewChangeTrackerDB(db *memdb.MemDB, publisher *stream.EventBroker, changesFn changeProcessor) *changeTrackerDB {
	return &changeTrackerDB{
		memdb:          db,
		publisher:      publisher,
		processChanges: changesFn,
	}
}

type changeProcessor func(ReadTxn, Changes) *structs.Events

func noOpProcessChanges(ReadTxn, Changes) *structs.Events { return nil }

// ReadTxn returns a read-only transaction which behaves exactly the same as
// memdb.Txn
//
// TODO: this could return a regular memdb.Txn if all the state functions accepted
// the ReadTxn interface
func (c *changeTrackerDB) ReadTxn() *txn {
	return &txn{Txn: c.memdb.Txn(false)}
}

// WriteTxn returns a wrapped memdb.Txn suitable for writes to the state store.
// It will track changes and publish events for the changes when Commit
// is called.
//
// The idx argument must be the index of the current Raft operation. Almost
// all mutations to state should happen as part of a raft apply so the index of
// the log being applied can be passed to WriteTxn.
// The exceptional cases are transactions that are executed on an empty
// memdb.DB as part of Restore, and those executed by tests where we insert
// data directly into the DB. These cases may use WriteTxnRestore.
func (c *changeTrackerDB) WriteTxn(idx uint64) *txn {
	t := &txn{
		Txn:     c.memdb.Txn(true),
		Index:   idx,
		publish: c.publish,
		msgType: structs.IgnoreUnknownTypeFlag, // The zero value of structs.MessageType is noderegistration.
	}
	t.Txn.TrackChanges()
	return t
}

func (c *changeTrackerDB) WriteTxnMsgT(msgType structs.MessageType, idx uint64) *txn {
	t := &txn{
		msgType: msgType,
		Txn:     c.memdb.Txn(true),
		Index:   idx,
		publish: c.publish,
	}
	t.Txn.TrackChanges()
	return t
}

func (c *changeTrackerDB) publish(changes Changes) (*structs.Events, error) {
	readOnlyTx := c.memdb.Txn(false)
	defer readOnlyTx.Abort()

	events := c.processChanges(readOnlyTx, changes)
	if events != nil {
		c.publisher.Publish(events)
	}

	return events, nil
}

// WriteTxnRestore returns a wrapped RW transaction that does NOT have change
// tracking enabled. This should only be used in Restore where we need to
// replace the entire contents of the Store without a need to track the changes.
// WriteTxnRestore uses a zero index since the whole restore doesn't really occur
// at one index - the effect is to write many values that were previously
// written across many indexes.
func (c *changeTrackerDB) WriteTxnRestore() *txn {
	return &txn{
		Txn:   c.memdb.Txn(true),
		Index: 0,
	}
}

// txn wraps a memdb.Txn to capture changes and send them to the EventBroker.
//
// This can not be done with txn.Defer because the callback passed to Defer is
// invoked after commit completes, and because the callback can not return an
// error. Any errors from the callback would be lost,  which would result in a
// missing change event, even though the state store had changed.
type txn struct {
	// msgType is used to inform event sourcing which type of event to create
	msgType structs.MessageType

	*memdb.Txn
	// Index in raft where the write is occurring. The value is zero for a
	// read-only, or WriteTxnRestore transaction.
	// Index is stored so that it may be passed along to any subscribers as part
	// of a change event.
	Index   uint64
	publish func(changes Changes) (*structs.Events, error)
}

// Commit first pushes changes to EventBroker, then calls Commit on the
// underlying transaction.
//
// Note that this function, unlike memdb.Txn, returns an error which must be checked
// by the caller. A non-nil error indicates that a commit failed and was not
// applied.
func (tx *txn) Commit() error {
	// publish may be nil if this is a read-only or WriteTxnRestore transaction.
	// In those cases changes should also be empty, and there will be nothing
	// to publish.
	if tx.publish != nil {
		changes := Changes{
			Index:   tx.Index,
			Changes: tx.Txn.Changes(),
			MsgType: tx.MsgType(),
		}
		_, err := tx.publish(changes)
		if err != nil {
			return err
		}
	}

	tx.Txn.Commit()
	return nil
}

// MsgType returns a MessageType from the txn's context.
// If the context is empty or the value isn't set IgnoreUnknownTypeFlag will
// be returned to signal that the MsgType is unknown.
func (tx *txn) MsgType() structs.MessageType {
	return tx.msgType
}

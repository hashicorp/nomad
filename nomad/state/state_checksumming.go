package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/hashstructure"
)

var (
	errMsgBadHash                    = "could not calculate hash of object in %q table: %v"
	errMsgStateStoreChecksumMismatch = "detected state store corruption in %q table, object: %#v"
	errMsgStateStoreChecksumMissing  = "detected missing checksum in %q table"
)

type Checksum struct {
	Table string
	Hash  uint64
}

type checksummingDB struct {
	memdb   MemDBWrapper
	enabled bool
}

func NewChecksummingDB(db MemDBWrapper, enabled bool) *checksummingDB {
	return &checksummingDB{
		memdb:   db,
		enabled: enabled,
	}
}

// ReadTxn ... TODO
func (c *checksummingDB) ReadTxn() Txn {
	return &checksummedTxn{Txn: c.memdb.ReadTxn()}
}

// WriteTxn ... TODO
func (c *checksummingDB) WriteTxn(idx uint64) Txn {
	t := &checksummedTxn{
		Txn:     c.memdb.WriteTxn(idx),
		index:   idx,
		msgType: structs.IgnoreUnknownTypeFlag, // The zero value of structs.MessageType is noderegistration.
	}
	t.Txn.TrackChanges()
	return t
}

// WriteTxnMsgT ... TODO
func (c *checksummingDB) WriteTxnMsgT(msgType structs.MessageType, idx uint64) Txn {
	t := &checksummedTxn{
		msgType: msgType,
		index:   idx,
		Txn:     c.memdb.WriteTxnMsgT(msgType, idx),
	}
	t.Txn.TrackChanges()
	return t
}

// WriteTxnRestore ... TODO
func (c *checksummingDB) WriteTxnRestore() Txn {
	return &checksummedTxn{Txn: c.memdb.WriteTxnRestore()}

}

// Publisher ... TODO
func (c *checksummingDB) Publisher() *stream.EventBroker {
	return nil
}

// Snapshot ... TODO
func (c *checksummingDB) Snapshot() *memdb.MemDB {
	return c.memdb.Snapshot()
}

// checksummedTxn ... TODO
type checksummedTxn struct {
	msgType structs.MessageType
	index   uint64
	Txn     // wrap the inner Txn
}

// Delete ... TODO
func (tx *checksummedTxn) Delete(table string, obj any) error {
	if err := tx.verifyChecksum(table, obj); err != nil {
		return err
	}
	return tx.Txn.Delete(table, obj)
}

func (tx *checksummedTxn) verifyChecksum(table string, obj any) error {
	if obj == nil || table == tableIndex {
		return nil
	}
	hash, err := hashstructure.Hash(obj, nil)
	if err != nil {
		return fmt.Errorf(errMsgBadHash, table, err)
	}
	raw, err := tx.Txn.First(TableChecksums, indexID, table, hash)
	if err != nil {
		return err // unreachable
	}
	if raw == nil {
		// if our checksum doesn't match we won't find anything for this hash
		return fmt.Errorf(errMsgStateStoreChecksumMismatch, table, obj)
	}
	return nil
}

type ChecksumIterator struct {
	inner memdb.ResultIterator

	results []any
	index   int
}

func (tx *checksummedTxn) newIterator(table string, iter memdb.ResultIterator) (memdb.ResultIterator, error) {
	checksumIter := &ChecksumIterator{inner: iter, results: []any{}}
	for {
		obj := iter.Next()
		if obj == nil {
			break
		}
		err := tx.verifyChecksum(table, obj)
		if err != nil {
			return nil, err
		}
		checksumIter.results = append(checksumIter.results, obj)
	}
	return checksumIter, nil
}

func (iter *ChecksumIterator) Next() any {
	if len(iter.results) > iter.index {
		result := iter.results[iter.index]
		iter.index++
		return result
	}
	return nil
}

func (iter *ChecksumIterator) WatchCh() <-chan struct{} {
	return iter.inner.WatchCh()
}

// DeleteAll ... TODO
func (tx *checksummedTxn) DeleteAll(table, index string, args ...any) (int, error) {
	// TODO: checksum on delete
	return tx.Txn.DeleteAll(table, index, args...)
}

// DeletePrefix ... TODO
func (tx *checksummedTxn) DeletePrefix(table, prefix_index, prefix string) (bool, error) {
	// TODO: checksum on delete
	return tx.Txn.DeletePrefix(table, prefix_index, prefix)
}

// Get ... TODO
func (tx *checksummedTxn) Get(table, index string, args ...any) (memdb.ResultIterator, error) {
	iter, err := tx.Txn.Get(table, index, args...)
	if err != nil {
		return nil, err
	}
	return tx.newIterator(table, iter)
}

// GetReverse ... TODO
func (tx *checksummedTxn) GetReverse(table, index string, args ...any) (memdb.ResultIterator, error) {
	iter, err := tx.Txn.GetReverse(table, index, args...)
	if err != nil {
		return nil, err
	}
	return tx.newIterator(table, iter)
}

// First ... TODO
func (tx *checksummedTxn) First(table, index string, args ...any) (any, error) {
	obj, err := tx.Txn.First(table, index, args...)
	if err != nil { // unreachable
		return obj, err
	}
	if obj == nil {
		return nil, nil
	}
	if err := tx.verifyChecksum(table, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// FirstWatch ... TODO
func (tx *checksummedTxn) FirstWatch(table, index string, args ...any) (<-chan struct{}, any, error) {
	ch, obj, err := tx.Txn.FirstWatch(table, index, args...)
	if err != nil {
		return ch, nil, err
	}
	if obj == nil {
		return ch, nil, nil
	}
	if err := tx.verifyChecksum(table, obj); err != nil {
		return ch, nil, err
	}
	return ch, obj, nil
}

// Insert ... TODO
func (tx *checksummedTxn) Insert(table string, obj any) error {

	if table == tableIndex {
		// no need to checksum the index table
		return tx.Txn.Insert(table, obj)
	}

	hash, err := hashstructure.Hash(obj, nil)
	if err != nil {
		return err
	}

	err = tx.Txn.Insert(TableChecksums, Checksum{Table: table, Hash: hash})
	if err != nil {
		return err
	}

	return tx.Txn.Insert(table, obj)
}

// MsgType ... TODO
func (tx *checksummedTxn) MsgType() structs.MessageType {
	return tx.msgType
}

// Index ... TODO
func (tx *checksummedTxn) Index() uint64 {
	return tx.index
}

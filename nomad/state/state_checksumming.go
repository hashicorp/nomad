package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/hashstructure"
)

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
	Txn

	index uint64
}

func (tx *checksummedTxn) Get(table, index string, args ...any) (memdb.ResultIterator, error) {
	return tx.Txn.Get(table, index, args...)
}

func (tx *checksummedTxn) Insert(table string, obj any) error {
	hash, err := hashstructure.Hash(obj, nil)
	if err != nil {
		return err
	}
	fmt.Println("checkedsummedTxn.Insert: ", table, hash)

	return tx.Txn.Insert(table, obj)
}

func (tx *checksummedTxn) Delete(table string, obj any) error {
	hash, err := hashstructure.Hash(obj, nil)
	if err != nil {
		return err
	}
	fmt.Println("checkedsummedTxn.Delete: ", table, hash)

	return tx.Txn.Delete(table, obj)
}

func (tx *checksummedTxn) MsgType() structs.MessageType {
	return tx.msgType
}

func (tx *checksummedTxn) Index() uint64 {
	return tx.index
}

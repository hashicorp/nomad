// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"
)

var (
	// Ensure the required memdb interfaces are met at compile time.
	_ memdb.Indexer       = SingleIndexer{}
	_ memdb.SingleIndexer = SingleIndexer{}
)

// SingleIndexer implements both memdb.Indexer and memdb.SingleIndexer. It may
// be used in a memdb.IndexSchema to specify functions that generate the index
// value for memdb.Txn operations.
type SingleIndexer struct {

	// readIndex is used by memdb for Txn.Get, Txn.First, and other operations
	// that read data.
	ReadIndex

	// writeIndex is used by memdb for Txn.Insert, Txn.Delete, and other
	// operations that write data to the index.
	WriteIndex
}

// ReadIndex implements memdb.Indexer. It exists so that a function can be used
// to provide the interface.
//
// Unlike memdb.Indexer, a readIndex function accepts only a single argument. To
// generate an index from multiple values, use a struct type with multiple fields.
type ReadIndex func(arg any) ([]byte, error)

func (f ReadIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("index supports only a single arg")
	}
	return f(args[0])
}

var ErrMissingValueForIndex = fmt.Errorf("object is missing a value for this index")

// WriteIndex implements memdb.SingleIndexer. It exists so that a function
// can be used to provide this interface.
//
// Instead of a bool return value, writeIndex expects errMissingValueForIndex to
// indicate that an index could not be build for the object. It will translate
// this error into a false value to satisfy the memdb.SingleIndexer interface.
type WriteIndex func(raw any) ([]byte, error)

func (f WriteIndex) FromObject(raw any) (bool, []byte, error) {
	v, err := f(raw)
	if errors.Is(err, ErrMissingValueForIndex) {
		return false, nil, nil
	}
	return err == nil, v, err
}

// IndexBuilder is a buffer used to construct memdb index values.
type IndexBuilder bytes.Buffer

// Bytes returns the stored IndexBuilder value as a byte array.
func (b *IndexBuilder) Bytes() []byte { return (*bytes.Buffer)(b).Bytes() }

// Time is used to write the passed time into the IndexBuilder for use as a
// memdb index value.
func (b *IndexBuilder) Time(t time.Time) {
	val := t.Unix()
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(val))
	(*bytes.Buffer)(b).Write(buf)
}

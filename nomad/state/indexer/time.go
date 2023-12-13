// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexer

import (
	"fmt"
	"time"
)

type TimeQuery struct {
	Value time.Time
}

// IndexFromTimeQuery can be used as a memdb.Indexer query via ReadIndex and
// allows querying by time.
func IndexFromTimeQuery(arg any) ([]byte, error) {
	p, ok := arg.(*TimeQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for TimeQuery index", arg)
	}

	// Construct the index value and return the byte array representation of
	// the time value.
	var b IndexBuilder
	b.Time(p.Value)
	return b.Bytes(), nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rafttools

import (
	"fmt"

	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

func RaftState(p string) (store *raftboltdb.BoltStore, firstIdx uint64, lastIdx uint64, err error) {
	s, err := raftboltdb.NewBoltStore(p)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to open raft logs: %v", err)
	}

	firstIdx, err = s.FirstIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch first index: %v", err)
	}

	lastIdx, err = s.LastIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch last index: %v", err)
	}

	return s, firstIdx, lastIdx, nil
}

package state

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SecureVariables queries all the variables and is used only for
// snapshot/restore and key rotation
func (s *StateStore) SecureVariables(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableSecureVariables, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}


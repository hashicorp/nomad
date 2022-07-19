package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// ACLTokensByExpired returns an array accessor IDs of expired ACL tokens.
// Their expiration is determined against the passed time.Time value.
//
// The function handles global and local tokens independently as determined by
// the global boolean argument. The number of returned IDs can be limited by
// the max integer, which is useful to limit the number of tokens we attempt to
// delete in a single transaction.
func (s *StateStore) ACLTokensByExpired(global bool) (memdb.ResultIterator, error) {
	tnx := s.db.ReadTxn()

	iter, err := tnx.Get("acl_token", expiresIndexName(global))
	if err != nil {
		return nil, fmt.Errorf("failed acl token listing: %v", err)
	}
	return iter, nil
}

// expiresIndexName is a helper function to identify the correct ACL token
// table expiry index to use.
func expiresIndexName(global bool) string {
	if global {
		return indexExpiresGlobal
	}
	return indexExpiresLocal
}

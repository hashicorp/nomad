package state

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ACLTokensByExpired returns an array accessor IDs of expired ACL tokens.
// Their expiration is determined against the passed time.Time value.
//
// The function handles global and local tokens independently as determined by
// the global boolean argument. The number of returned IDs can be limited by
// the max integer, which is useful to limit the number of tokens we attempt to
// delete in a single transaction.
func (s *StateStore) ACLTokensByExpired(global bool, now time.Time, max int) ([]string, error) {
	tnx := s.db.ReadTxn()

	iter, err := tnx.Get("acl_token", expiresIndexName(global))
	if err != nil {
		return nil, fmt.Errorf("failed acl token listing: %v", err)
	}

	var (
		accessorIDs []string
		num         int
	)

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)

		// The indexes mean if we come across an unexpired token, we can exit
		// as we have found all currently expired tokens.
		if !token.IsExpired(now) {
			return accessorIDs, nil
		}

		accessorIDs = append(accessorIDs, token.AccessorID)

		// Increment the counter. If this is at or above our limit, we return
		// what we have so far.
		num++
		if num >= max {
			return accessorIDs, nil
		}
	}

	return accessorIDs, nil
}

// expiresIndexName is a helper function to identify the correct ACL token
// table expiry index to use.
func expiresIndexName(global bool) string {
	if global {
		return indexExpiresGlobal
	}
	return indexExpiresLocal
}

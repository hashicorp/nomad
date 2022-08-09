package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
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

// UpsertACLRoles is used to insert a number of ACL roles into the state store.
// It uses a single write transaction for efficiency, however, any error means
// no entries will be committed.
func (s *StateStore) UpsertACLRoles(
	msgType structs.MessageType, index uint64, roles []*structs.ACLRole) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// updated tracks whether any inserts have been made. This allows us to
	// skip updating the index table if we do not need to.
	var updated bool

	// Iterate the array of roles. In the event of a single error, all inserts
	// fail via the txn.Abort() defer.
	for _, role := range roles {

		roleUpdated, err := s.upsertACLRoleTxn(index, txn, role)
		if err != nil {
			return err
		}

		// Ensure we track whether any inserts have been made.
		updated = updated || roleUpdated
	}

	// If we did not perform any inserts, exit early.
	if !updated {
		return nil
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLRoles, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertACLRoleTxn inserts a single ACL role into the state store using the
// provided write transaction. It is the responsibility of the caller to update
// the index table.
func (s *StateStore) upsertACLRoleTxn(
	index uint64, txn *txn, role *structs.ACLRole) (bool, error) {

	// Ensure the role hash is not zero to provide defense in depth. This
	// should be done outside the state store, so we do not spend time here
	// and thus Raft, when it, can be avoided.
	if len(role.Hash) == 0 {
		role.SetHash()
	}

	// This validation also happens within the RPC handler, but Raft latency
	// could mean that by the time the state call is invoked, another Raft
	// update has deleted policies detailed in role. Therefore, check again
	// while in our write txn.
	if err := s.validateACLRolePolicyLinksTxn(txn, role); err != nil {
		return false, err
	}

	existing, err := txn.First(TableACLRoles, indexID, role.ID)
	if err != nil {
		return false, fmt.Errorf("ACL role lookup failed: %v", err)
	}

	// Set up the indexes correctly to ensure existing indexes are maintained.
	if existing != nil {
		exist := existing.(*structs.ACLRole)
		if exist.Equals(role) {
			return false, nil
		}
		role.CreateIndex = exist.CreateIndex
		role.ModifyIndex = index
	} else {
		role.CreateIndex = index
		role.ModifyIndex = index
	}

	// Insert the role into the table.
	if err := txn.Insert(TableACLRoles, role); err != nil {
		return false, fmt.Errorf("ACL role insert failed: %v", err)
	}
	return true, nil
}

// ValidateACLRolePolicyLinks ensures all ACL policies linked to from the ACL
// role exist within state.
func (s *StateStore) ValidateACLRolePolicyLinks(role *structs.ACLRole) error {
	txn := s.db.ReadTxn()
	return s.validateACLRolePolicyLinksTxn(txn, role)
}

// validateACLRolePolicyLinksTxn is the same as ValidateACLRolePolicyLinks but
// allows callers to pass their own transaction.
func (s *StateStore) validateACLRolePolicyLinksTxn(txn *txn, role *structs.ACLRole) error {
	for _, policyLink := range role.Policies {
		_, existing, err := txn.FirstWatch("acl_policy", indexID, policyLink.Name)
		if err != nil {
			return fmt.Errorf("ACL policy lookup failed: %v", err)
		}
		if existing == nil {
			return errors.New("ACL policy not found")
		}
	}
	return nil
}

// DeleteACLRolesByID is responsible for batch deleting ACL roles based on
// their ID. It uses a single write transaction for efficiency, however, any
// error means no entries will be committed. An error is produced if a role is
// not found within state which has been passed within the array.
func (s *StateStore) DeleteACLRolesByID(
	msgType structs.MessageType, index uint64, roleIDs []string) error {

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	for _, roleID := range roleIDs {

		existing, err := txn.First(TableACLRoles, indexID, roleID)
		if err != nil {
			return fmt.Errorf("ACL role lookup failed: %v", err)
		}
		if existing == nil {
			return errors.New("ACL role not found")
		}

		// Delete the existing entry from the table.
		if err := txn.Delete(TableACLRoles, existing); err != nil {
			return fmt.Errorf("ACL role deletion failed: %v", err)
		}
	}

	// Update the index table to indicate an update has occurred.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLRoles, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// GetACLRoles returns an iterator that contains all ACL roles stored within
// state.
func (s *StateStore) GetACLRoles(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table to get all ACL roles.
	iter, err := txn.Get(TableACLRoles, indexID)
	if err != nil {
		return nil, fmt.Errorf("ACL role lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetACLRoleByID returns a single ACL role specified by the input ID. The role
// object will be nil, if no matching entry was found; it is the responsibility
// of the caller to check for this.
func (s *StateStore) GetACLRoleByID(ws memdb.WatchSet, roleID string) (*structs.ACLRole, error) {
	txn := s.db.ReadTxn()

	// Perform the ACL role lookup using the "id" index.
	watchCh, existing, err := txn.FirstWatch(TableACLRoles, indexID, roleID)
	if err != nil {
		return nil, fmt.Errorf("ACL role lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLRole), nil
	}
	return nil, nil
}

// GetACLRoleByName returns a single ACL role specified by the input name. The
// role object will be nil, if no matching entry was found; it is the
// responsibility of the caller to check for this.
func (s *StateStore) GetACLRoleByName(ws memdb.WatchSet, roleName string) (*structs.ACLRole, error) {
	txn := s.db.ReadTxn()

	// Perform the ACL role lookup using the "name" index.
	watchCh, existing, err := txn.FirstWatch(TableACLRoles, indexName, roleName)
	if err != nil {
		return nil, fmt.Errorf("ACL role lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLRole), nil
	}
	return nil, nil
}

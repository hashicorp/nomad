//go:build !ent
// +build !ent

package state

func (s *StateStore) enforceVariablesQuota(_ uint64, _ WriteTxn, _ string, _ int64) error {
	return nil
}

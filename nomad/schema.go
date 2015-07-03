package nomad

import "github.com/hashicorp/go-memdb"

// stateStoreSchema is used to return the schema for the state store
func stateStoreSchema() *memdb.DBSchema {
	return nil
}

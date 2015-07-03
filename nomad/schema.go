package nomad

import "github.com/hashicorp/go-memdb"

// stateStoreSchema is used to return the schema for the state store
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
	}

	// Add each of the tables
	nodeSchema := nodeTableSchema()
	db.Tables[nodeSchema.Name] = nodeSchema

	return db
}

// nodeTableSchema returns the MemDB schema for the nodes table.
// This table is used to store all the client nodes that are registered.
func nodeTableSchema() *memdb.TableSchema {
	table := &memdb.TableSchema{
		Name: "nodes",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for node management
			// and simple direct lookup. ID is required to be
			// unique.
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ID",
					Lowercase: true,
				},
			},

			// DC status is a compound index on both the
			// datacenter and the node status. This allows
			// us to filter to a set of eligible nodes more
			// quickly for selection.
			"dc-status": &memdb.IndexSchema{
				Name:         "dc-status",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Datacenter",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Status",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
	return table
}

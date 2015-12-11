package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// stateStoreSchema is used to return the schema for the state store
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
	}

	// Collect all the schemas that are needed
	schemas := []func() *memdb.TableSchema{
		indexTableSchema,
		nodeTableSchema,
		jobTableSchema,
		evalTableSchema,
		allocTableSchema,
	}

	// Add each of the tables
	for _, schemaFn := range schemas {
		schema := schemaFn()
		if _, ok := db.Tables[schema.Name]; ok {
			panic(fmt.Sprintf("duplicate table name: %s", schema.Name))
		}
		db.Tables[schema.Name] = schema
	}
	return db
}

// indexTableSchema is used for
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "index",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: true,
				},
			},
		},
	}
}

// nodeTableSchema returns the MemDB schema for the nodes table.
// This table is used to store all the client nodes that are registered.
func nodeTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
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
		},
	}
}

// jobTableSchema returns the MemDB schema for the jobs table.
// This table is used to store all the jobs that have been submitted.
func jobTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "jobs",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for job management
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
			"type": &memdb.IndexSchema{
				Name:         "type",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Type",
					Lowercase: false,
				},
			},
			"parent": &memdb.IndexSchema{
				Name:         "parent",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ParentID",
					Lowercase: true,
				},
			},
		},
	}
}

// evalTableSchema returns the MemDB schema for the eval table.
// This table is used to store all the evaluations that are pending
// or recently completed.
func evalTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "evals",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for direct lookup.
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			// Job index is used to lookup allocations by job
			"job": &memdb.IndexSchema{
				Name:         "job",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "JobID",
					Lowercase: true,
				},
			},
		},
	}
}

// allocTableSchema returns the MemDB schema for the allocation table.
// This table is used to store all the task allocations between task groups
// and nodes.
func allocTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "allocs",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is a UUID
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			// Node index is used to lookup allocations by node
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true, // Missing is allow for failed allocations
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "NodeID",
					Lowercase: true,
				},
			},

			// Job index is used to lookup allocations by job
			"job": &memdb.IndexSchema{
				Name:         "job",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "JobID",
					Lowercase: true,
				},
			},

			// Eval index is used to lookup allocations by eval
			"eval": &memdb.IndexSchema{
				Name:         "eval",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "EvalID",
				},
			},
		},
	}
}

package nomad

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
		nodeTableSchema,
		jobTableSchema,
		taskGroupTableSchema,
		taskTableSchema,
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
							Field: "Status",
						},
					},
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
					Field:     "Name",
					Lowercase: true,
				},
			},

			// Status is used to scan for jobs that are in need
			// of scheduling attention.
			"status": &memdb.IndexSchema{
				Name:         "status",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Status",
				},
			},
		},
	}
}

// taskGroupTableSchema returns the MemDB schema for the task group table.
// This table is used to store all the task groups belonging to a job.
func taskGroupTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "taskGroups",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is compount of {Job, Name}
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Job",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Name",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

// taskTableSchema returns the MemDB schema for the tasks table.
// This table is used to store all the task groups belonging to a job.
func taskTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "tasks",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is compount of {Job, TaskGroup, Name}
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Job",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "TaskGroup",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Name",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

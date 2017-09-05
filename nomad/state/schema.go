package state

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	schemaFactories SchemaFactories
	factoriesLock   sync.Mutex
)

// SchemaFactory is the factory method for returning a TableSchema
type SchemaFactory func() *memdb.TableSchema
type SchemaFactories []SchemaFactory

// RegisterSchemaFactories is used to register a table schema.
func RegisterSchemaFactories(factories ...SchemaFactory) {
	factoriesLock.Lock()
	defer factoriesLock.Unlock()
	schemaFactories = append(schemaFactories, factories...)
}

func GetFactories() SchemaFactories {
	return schemaFactories
}

func init() {
	// Register all schemas
	RegisterSchemaFactories([]SchemaFactory{
		indexTableSchema,
		nodeTableSchema,
		jobTableSchema,
		jobSummarySchema,
		jobVersionSchema,
		deploymentSchema,
		periodicLaunchTableSchema,
		evalTableSchema,
		allocTableSchema,
		vaultAccessorTableSchema,
		aclPolicyTableSchema,
		aclTokenTableSchema,
	}...)
}

// stateStoreSchema is used to return the schema for the state store
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
	}

	// Add each of the tables
	for _, schemaFn := range GetFactories() {
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
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
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
			// unique within a namespace.
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, ID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
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
			"gc": &memdb.IndexSchema{
				Name:         "gc",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: jobIsGCable,
				},
			},
			"periodic": &memdb.IndexSchema{
				Name:         "periodic",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: jobIsPeriodic,
				},
			},
		},
	}
}

// jobSummarySchema returns the memdb schema for the job summary table
func jobSummarySchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "job_summary",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
		},
	}
}

// jobVersionSchema returns the memdb schema for the job version table which
// keeps a historical view of job versions.
func jobVersionSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "job_version",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, ID, Version) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field:     "ID",
							Lowercase: true,
						},

						&memdb.UintFieldIndex{
							Field: "Version",
						},
					},
				},
			},
		},
	}
}

// jobIsGCable satisfies the ConditionalIndexFunc interface and creates an index
// on whether a job is eligible for garbage collection.
func jobIsGCable(obj interface{}) (bool, error) {
	j, ok := obj.(*structs.Job)
	if !ok {
		return false, fmt.Errorf("Unexpected type: %v", obj)
	}

	// If the job is periodic or parameterized it is only garbage collectable if
	// it is stopped.
	periodic := j.Periodic != nil && j.Periodic.Enabled
	parameterized := j.IsParameterized()
	if periodic || parameterized {
		return j.Stop, nil
	}

	// If the job isn't dead it isn't eligible
	if j.Status != structs.JobStatusDead {
		return false, nil
	}

	// Any job that is stopped is eligible for garbage collection
	if j.Stop {
		return true, nil
	}

	// Otherwise, only batch jobs are eligible because they complete on their
	// own without a user stopping them.
	if j.Type != structs.JobTypeBatch {
		return false, nil
	}

	return true, nil
}

// jobIsPeriodic satisfies the ConditionalIndexFunc interface and creates an index
// on whether a job is periodic.
func jobIsPeriodic(obj interface{}) (bool, error) {
	j, ok := obj.(*structs.Job)
	if !ok {
		return false, fmt.Errorf("Unexpected type: %v", obj)
	}

	if j.Periodic != nil && j.Periodic.Enabled == true {
		return true, nil
	}

	return false, nil
}

// deploymentSchema returns the MemDB schema tracking a job's deployments
func deploymentSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "deployment",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			"namespace": &memdb.IndexSchema{
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Job index is used to lookup deployments by job
			"job": &memdb.IndexSchema{
				Name:         "job",
				AllowMissing: false,
				Unique:       false,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
		},
	}
}

// periodicLaunchTableSchema returns the MemDB schema tracking the most recent
// launch time for a perioidic job.
func periodicLaunchTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "periodic_launch",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for job management
			// and simple direct lookup. ID is required to be
			// unique.
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
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

			"namespace": &memdb.IndexSchema{
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Job index is used to lookup allocations by job
			"job": &memdb.IndexSchema{
				Name:         "job",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field:     "JobID",
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

			"namespace": &memdb.IndexSchema{
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Node index is used to lookup allocations by node
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true, // Missing is allow for failed allocations
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "NodeID",
							Lowercase: true,
						},

						// Conditional indexer on if allocation is terminal
						&memdb.ConditionalIndex{
							Conditional: func(obj interface{}) (bool, error) {
								// Cast to allocation
								alloc, ok := obj.(*structs.Allocation)
								if !ok {
									return false, fmt.Errorf("wrong type, got %t should be Allocation", obj)
								}

								// Check if the allocation is terminal
								return alloc.TerminalStatus(), nil
							},
						},
					},
				},
			},

			// Job index is used to lookup allocations by job
			"job": &memdb.IndexSchema{
				Name:         "job",
				AllowMissing: false,
				Unique:       false,

				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
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

			// Deployment index is used to lookup allocations by deployment
			"deployment": &memdb.IndexSchema{
				Name:         "deployment",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "DeploymentID",
				},
			},
		},
	}
}

// vaultAccessorTableSchema returns the MemDB schema for the Vault Accessor
// Table. This table tracks Vault accessors for tokens created on behalf of
// allocations required Vault tokens.
func vaultAccessorTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "vault_accessors",
		Indexes: map[string]*memdb.IndexSchema{
			// The primary index is the accessor id
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Accessor",
				},
			},

			"alloc_id": &memdb.IndexSchema{
				Name:         "alloc_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},

			"node_id": &memdb.IndexSchema{
				Name:         "node_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodeID",
				},
			},
		},
	}
}

// aclPolicyTableSchema returns the MemDB schema for the policy table.
// This table is used to store the policies which are refrenced by tokens
func aclPolicyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl_policy",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
		},
	}
}

// aclTokenTableSchema returns the MemDB schema for the tokens table.
// This table is used to store the bearer tokens which are used to authenticate
func aclTokenTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl_token",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"secret": &memdb.IndexSchema{
				Name:         "secret",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "SecretID",
				},
			},
			"global": &memdb.IndexSchema{
				Name:         "global",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.FieldSetIndex{
					Field: "Global",
				},
			},
		},
	}
}

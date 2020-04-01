package state

import (
	"fmt"
	"sync"

	memdb "github.com/hashicorp/go-memdb"

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
		siTokenAccessorTableSchema,
		aclPolicyTableSchema,
		aclTokenTableSchema,
		autopilotConfigTableSchema,
		schedulerConfigTableSchema,
		clusterMetaTableSchema,
		csiVolumeTableSchema,
		csiPluginTableSchema,
		scalingPolicyTableSchema,
		scalingEventTableSchema,
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

// indexTableSchema is used for tracking the most recent index used for each table.
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "index",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"secret_id": {
				Name:         "secret_id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "SecretID",
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
			"id": {
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
			"type": {
				Name:         "type",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Type",
					Lowercase: false,
				},
			},
			"gc": {
				Name:         "gc",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: jobIsGCable,
				},
			},
			"periodic": {
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
			"id": {
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
			"id": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Job index is used to lookup deployments by job
			"job": {
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
// launch time for a periodic job.
func periodicLaunchTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "periodic_launch",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for job management
			// and simple direct lookup. ID is required to be
			// unique.
			"id": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Job index is used to lookup allocations by job
			"job": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// Node index is used to lookup allocations by node
			"node": {
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
			"job": {
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
			"eval": {
				Name:         "eval",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "EvalID",
				},
			},

			// Deployment index is used to lookup allocations by deployment
			"deployment": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Accessor",
				},
			},

			"alloc_id": {
				Name:         "alloc_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},

			"node_id": {
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

// siTokenAccessorTableSchema returns the MemDB schema for the Service Identity
// token accessor table. This table tracks accessors for tokens created on behalf
// of allocations with Consul connect enabled tasks that need SI tokens.
func siTokenAccessorTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: siTokenAccessorTable,
		Indexes: map[string]*memdb.IndexSchema{
			// The primary index is the accessor id
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "AccessorID",
				},
			},

			"alloc_id": {
				Name:         "alloc_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},

			"node_id": {
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
// This table is used to store the policies which are referenced by tokens
func aclPolicyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl_policy",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"secret": {
				Name:         "secret",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "SecretID",
				},
			},
			"global": {
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

// singletonRecord can be used to describe tables which should contain only 1 entry.
// Example uses include storing node config or cluster metadata blobs.
var singletonRecord = &memdb.ConditionalIndex{
	Conditional: func(interface{}) (bool, error) { return true, nil },
}

// schedulerConfigTableSchema returns the MemDB schema for the scheduler config table.
// This table is used to store configuration options for the scheduler
func schedulerConfigTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scheduler_config",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: true,
				Unique:       true,
				Indexer:      singletonRecord, // we store only 1 scheduler config
			},
		},
	}
}

// clusterMetaTableSchema returns the MemDB schema for the scheduler config table.
func clusterMetaTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "cluster_meta",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer:      singletonRecord, // we store only 1 cluster metadata
			},
		},
	}
}

// CSIVolumes are identified by id globally, and searchable by driver
func csiVolumeTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "csi_volumes",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
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
			"plugin_id": {
				Name:         "plugin_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "PluginID",
				},
			},
		},
	}
}

// CSIPlugins are identified by id globally, and searchable by driver
func csiPluginTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "csi_plugins",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
		},
	}
}

// StringFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type ScalingPolicyTargetFieldIndex struct {
	Field string
}

// FromObject is used to extract an index value from an
// object or to indicate that the index value is missing.
func (s *ScalingPolicyTargetFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	policy, ok := obj.(*structs.ScalingPolicy)
	if !ok {
		return false, nil, fmt.Errorf("object %#v is not a ScalingPolicy", obj)
	}

	if policy.Target == nil {
		return false, nil, nil
	}

	val, ok := policy.Target[s.Field]
	if !ok {
		return false, nil, nil
	}

	// Add the null character as a terminator
	val += "\x00"
	return true, []byte(val), nil
}

// FromArgs is used to build an exact index lookup based on arguments
func (s *ScalingPolicyTargetFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

// PrefixFromArgs returns a prefix that should be used for scanning based on the arguments
func (s *ScalingPolicyTargetFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// scalingPolicyTableSchema returns the MemDB schema for the policy table.
func scalingPolicyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scaling_policy",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for simple direct lookup.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// UUID is uniquely identifying
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
			// Target index is used for listing by namespace or job, or looking up a specific target.
			// A given task group can have only a single scaling policies, so this is guaranteed to be unique.
			"target": {
				Name:         "target",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, Job, Group) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&ScalingPolicyTargetFieldIndex{
							Field: "Namespace",
						},

						&ScalingPolicyTargetFieldIndex{
							Field: "Job",
						},

						&ScalingPolicyTargetFieldIndex{
							Field: "Group",
						},
					},
				},
			},
			// Used to filter by enabled
			"enabled": {
				Name:         "enabled",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.FieldSetIndex{
					Field: "Enabled",
				},
			},
		},
	}
}

// scalingEventTableSchema returns the memdb schema for job scaling events
func scalingEventTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scaling_event",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
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

			// TODO: need to figure out whether we want to index these or the jobs or ...
			// "error": {
			// 	Name:         "error",
			// 	AllowMissing: false,
			// 	Unique:       false,
			// 	Indexer: &memdb.FieldSetIndex{
			// 		Field: "Error",
			// 	},
			// },
		},
	}
}

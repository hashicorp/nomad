// +build pro ent

package state

import memdb "github.com/hashicorp/go-memdb"

func init() {
	// Register pro schemas
	RegisterSchemaFactories([]SchemaFactory{
		namespaceTableSchema,
	}...)
}

// namespaceTableSchema returns the MemDB schema for the namespace table.
func namespaceTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "namespaces",
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

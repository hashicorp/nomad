// +build pro ent

package state

import memdb "github.com/hashicorp/go-memdb"

const (
	TableNamespaces = "namespaces"
)

func init() {
	// Register pro schemas
	RegisterSchemaFactories([]SchemaFactory{
		namespaceTableSchema,
	}...)
}

// namespaceTableSchema returns the MemDB schema for the namespace table.
func namespaceTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableNamespaces,
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

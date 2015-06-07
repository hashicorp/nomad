package memdb

import "fmt"

// DBSchema contains the full database schema used for MemDB
type DBSchema struct {
	Tables []*TableSchema
}

// Validate is used to validate the database schema
func (s *DBSchema) Validate() error {
	if s == nil {
		return fmt.Errorf("missing schema")
	}
	if len(s.Tables) == 0 {
		return fmt.Errorf("no tables defined")
	}
	for _, table := range s.Tables {
		if err := table.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// TableSchema contains the schema for a single table
type TableSchema struct {
	Name    string
	Indexes []*IndexSchema
}

// Validate is used to validate the table schema
func (s *TableSchema) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing table name")
	}
	if len(s.Indexes) == 0 {
		return fmt.Errorf("missing table schemas for '%s'", s.Name)
	}
	for _, index := range s.Indexes {
		if err := index.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// IndexerFunc is used to extract an index value from an
// object or to indicate that the index value is missing.
type IndexerFunc func(interface{}) (bool, []byte, error)

// IndexSchema contains the schema for an index
type IndexSchema struct {
	Name         string
	AllowMissing bool
	Unique       bool
	Indexer      IndexerFunc
}

func (s *IndexSchema) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing index name")
	}
	if s.Indexer == nil {
		return fmt.Errorf("missing index function for '%s'", s.Name)
	}
	return nil
}

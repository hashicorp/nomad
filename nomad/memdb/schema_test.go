package memdb

import "testing"

func testValidSchema() *DBSchema {
	return &DBSchema{
		Tables: []*TableSchema{
			&TableSchema{
				Name: "main",
				Indexes: []*IndexSchema{
					&IndexSchema{
						Name:    "id",
						Indexer: StringFieldIndex("ID", false),
					},
				},
			},
		},
	}
}

func TestDBSchema_Validate(t *testing.T) {
	s := &DBSchema{}
	err := s.Validate()
	if err == nil {
		t.Fatalf("should not validate, empty")
	}

	s.Tables = []*TableSchema{
		&TableSchema{},
	}
	err = s.Validate()
	if err == nil {
		t.Fatalf("should not validate, no table name")
	}

	valid := testValidSchema()
	err = valid.Validate()
	if err != nil {
		t.Fatalf("should validate: %v", err)
	}
}

package memdb

import "testing"

// Test that multiple concurrent transactions are isolated from each other
func TestTxn_Isolation(t *testing.T) {
	db := testDB(t)
	txn1 := db.Txn(true)

	obj := &TestObject{
		ID:  "my-object",
		Foo: "abc",
	}
	obj2 := &TestObject{
		ID:  "my-cool-thing",
		Foo: "xyz",
	}
	obj3 := &TestObject{
		ID:  "my-other-cool-thing",
		Foo: "xyz",
	}

	err := txn1.Insert("main", obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = txn1.Insert("main", obj2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = txn1.Insert("main", obj3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Results should show up in this transaction
	raw, err := txn1.First("main", "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if raw == nil {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a new transaction, current one is NOT committed
	txn2 := db.Txn(false)

	// Nothing should show up in this transaction
	raw, err = txn2.First("main", "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if raw != nil {
		t.Fatalf("bad: %#v", raw)
	}

	// Commit txn1, txn2 should still be isolated
	txn1.Commit()

	// Nothing should show up in this transaction
	raw, err = txn2.First("main", "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if raw != nil {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a new txn
	txn3 := db.Txn(false)

	// Results should show up in this transaction
	raw, err = txn3.First("main", "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if raw == nil {
		t.Fatalf("bad: %#v", raw)
	}
}

// Test that an abort clears progress
func TestTxn_Abort(t *testing.T) {
	db := testDB(t)
	txn1 := db.Txn(true)

	obj := &TestObject{
		ID:  "my-object",
		Foo: "abc",
	}
	obj2 := &TestObject{
		ID:  "my-cool-thing",
		Foo: "xyz",
	}
	obj3 := &TestObject{
		ID:  "my-other-cool-thing",
		Foo: "xyz",
	}

	err := txn1.Insert("main", obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = txn1.Insert("main", obj2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = txn1.Insert("main", obj3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Abort the txn
	txn1.Abort()
	txn1.Commit()

	// Create a new transaction
	txn2 := db.Txn(false)

	// Nothing should show up in this transaction
	raw, err := txn2.First("main", "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if raw != nil {
		t.Fatalf("bad: %#v", raw)
	}
}

func TestComplexDB(t *testing.T) {
	db := testComplexDB(t)
	testPopulateData(t, db)
	txn := db.Txn(false) // read only

	// Get using a full name
	raw, err := txn.First("people", "name", "Armon", "Dadgar")
	noErr(t, err)
	if raw == nil {
		t.Fatalf("should get person")
	}

	// Get using a prefix
	raw, err = txn.First("people", "name_prefix", "Armon")
	noErr(t, err)
	if raw == nil {
		t.Fatalf("should get person")
	}

	// Where in the world is mitchell hashimoto?
	raw, err = txn.First("people", "name_prefix", "Mitchell")
	noErr(t, err)
	if raw == nil {
		t.Fatalf("should get person")
	}

	person := raw.(*TestPerson)
	if person.First != "Mitchell" {
		t.Fatalf("wrong person!")
	}

	raw, err = txn.First("visits", "id_prefix", person.ID)
	noErr(t, err)
	if raw == nil {
		t.Fatalf("should get visit")
	}

	visit := raw.(*TestVisit)

	raw, err = txn.First("places", "id", visit.Place)
	noErr(t, err)
	if raw == nil {
		t.Fatalf("should get place")
	}

	place := raw.(*TestPlace)
	if place.Name != "Maui" {
		t.Fatalf("bad place (but isn't anywhere else really?): %v", place)
	}
}

func testPopulateData(t *testing.T, db *MemDB) {
	// Start write txn
	txn := db.Txn(true)

	// Create some data
	person1 := testPerson()
	person2 := testPerson()
	person2.First = "Mitchell"
	person2.Last = "Hashimoto"

	place1 := testPlace()
	place2 := testPlace()
	place2.Name = "Maui"

	visit1 := &TestVisit{person1.ID, place1.ID}
	visit2 := &TestVisit{person2.ID, place2.ID}

	// Insert it all
	noErr(t, txn.Insert("people", person1))
	noErr(t, txn.Insert("people", person2))
	noErr(t, txn.Insert("places", place1))
	noErr(t, txn.Insert("places", place2))
	noErr(t, txn.Insert("visits", visit1))
	noErr(t, txn.Insert("visits", visit2))

	// Commit
	txn.Commit()
}

func noErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

type TestPerson struct {
	ID    string
	First string
	Last  string
}

type TestPlace struct {
	ID   string
	Name string
}

type TestVisit struct {
	Person string
	Place  string
}

func testComplexSchema() *DBSchema {
	return &DBSchema{
		Tables: map[string]*TableSchema{
			"people": &TableSchema{
				Name: "people",
				Indexes: map[string]*IndexSchema{
					"id": &IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &UUIDFieldIndex{Field: "ID"},
					},
					"name": &IndexSchema{
						Name:   "name",
						Unique: true,
						Indexer: &CompoundIndex{
							Indexes: []Indexer{
								&StringFieldIndex{Field: "First"},
								&StringFieldIndex{Field: "Last"},
							},
						},
					},
				},
			},
			"places": &TableSchema{
				Name: "places",
				Indexes: map[string]*IndexSchema{
					"id": &IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &UUIDFieldIndex{Field: "ID"},
					},
					"name": &IndexSchema{
						Name:    "name",
						Unique:  true,
						Indexer: &StringFieldIndex{Field: "Name"},
					},
				},
			},
			"visits": &TableSchema{
				Name: "visits",
				Indexes: map[string]*IndexSchema{
					"id": &IndexSchema{
						Name:   "id",
						Unique: true,
						Indexer: &CompoundIndex{
							Indexes: []Indexer{
								&UUIDFieldIndex{Field: "Person"},
								&UUIDFieldIndex{Field: "Place"},
							},
						},
					},
				},
			},
		},
	}
}

func testComplexDB(t *testing.T) *MemDB {
	db, err := NewMemDB(testComplexSchema())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return db
}

func testPerson() *TestPerson {
	_, uuid := generateUUID()
	obj := &TestPerson{
		ID:    uuid,
		First: "Armon",
		Last:  "Dadgar",
	}
	return obj
}

func testPlace() *TestPlace {
	_, uuid := generateUUID()
	obj := &TestPlace{
		ID:   uuid,
		Name: "HashiCorp",
	}
	return obj
}

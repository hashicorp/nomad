package memdb

import "testing"

type TestObject struct {
	Foo   string
	Bar   int
	Baz   string
	Empty string
}

func testObj() *TestObject {
	obj := &TestObject{
		Foo: "Testing",
		Bar: 42,
		Baz: "yep",
	}
	return obj
}

func TestStringFieldIndex(t *testing.T) {
	obj := testObj()
	indexer := StringFieldIndex("Foo", false)

	ok, val, err := indexer(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "Testing" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	lower := StringFieldIndex("Foo", true)
	ok, val, err = lower(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "testing" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	badField := StringFieldIndex("NA", true)
	ok, val, err = badField(obj)
	if err == nil {
		t.Fatalf("should get error")
	}

	emptyField := StringFieldIndex("Empty", true)
	ok, val, err = emptyField(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("should not ok")
	}
}

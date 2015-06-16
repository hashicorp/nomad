package memdb

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"testing"
)

type TestObject struct {
	ID    string
	Foo   string
	Bar   int
	Baz   string
	Empty string
}

func testObj() *TestObject {
	obj := &TestObject{
		ID:  "my-cool-obj",
		Foo: "Testing",
		Bar: 42,
		Baz: "yep",
	}
	return obj
}

func TestStringFieldIndex_FromObject(t *testing.T) {
	obj := testObj()
	indexer := StringFieldIndex{"Foo", false}

	ok, val, err := indexer.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "Testing\x00" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	lower := StringFieldIndex{"Foo", true}
	ok, val, err = lower.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "testing\x00" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	badField := StringFieldIndex{"NA", true}
	ok, val, err = badField.FromObject(obj)
	if err == nil {
		t.Fatalf("should get error")
	}

	emptyField := StringFieldIndex{"Empty", true}
	ok, val, err = emptyField.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("should not ok")
	}
}

func TestStringFieldIndex_FromArgs(t *testing.T) {
	indexer := StringFieldIndex{"Foo", false}
	_, err := indexer.FromArgs()
	if err == nil {
		t.Fatalf("should get err")
	}

	_, err = indexer.FromArgs(42)
	if err == nil {
		t.Fatalf("should get err")
	}

	val, err := indexer.FromArgs("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "foo\x00" {
		t.Fatalf("foo")
	}

	lower := StringFieldIndex{"Foo", true}
	val, err = lower.FromArgs("Foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "foo\x00" {
		t.Fatalf("foo")
	}
}

func TestStringFieldIndex_PrefixFromArgs(t *testing.T) {
	indexer := StringFieldIndex{"Foo", false}
	_, err := indexer.FromArgs()
	if err == nil {
		t.Fatalf("should get err")
	}

	_, err = indexer.PrefixFromArgs(42)
	if err == nil {
		t.Fatalf("should get err")
	}

	val, err := indexer.PrefixFromArgs("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "foo" {
		t.Fatalf("foo")
	}

	lower := StringFieldIndex{"Foo", true}
	val, err = lower.PrefixFromArgs("Foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "foo" {
		t.Fatalf("foo")
	}
}

func TestUUIDFeldIndex_parseString(t *testing.T) {
	u := &UUIDFieldIndex{}
	_, err := u.parseString("invalid")
	if err == nil {
		t.Fatalf("should error")
	}

	buf, uuid := generateUUID()

	out, err := u.parseString(uuid)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !bytes.Equal(out, buf) {
		t.Fatalf("bad: %#v %#v", out, buf)
	}
}

func TestUUIDFieldIndex_FromObject(t *testing.T) {
	obj := testObj()
	uuidBuf, uuid := generateUUID()
	obj.Foo = uuid
	indexer := &UUIDFieldIndex{"Foo"}

	ok, val, err := indexer.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(uuidBuf, val) {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	badField := &UUIDFieldIndex{"NA"}
	ok, val, err = badField.FromObject(obj)
	if err == nil {
		t.Fatalf("should get error")
	}

	emptyField := &UUIDFieldIndex{"Empty"}
	ok, val, err = emptyField.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("should not ok")
	}
}

func TestUUIDFieldIndex_FromArgs(t *testing.T) {
	indexer := &UUIDFieldIndex{"Foo"}
	_, err := indexer.FromArgs()
	if err == nil {
		t.Fatalf("should get err")
	}

	_, err = indexer.FromArgs(42)
	if err == nil {
		t.Fatalf("should get err")
	}

	uuidBuf, uuid := generateUUID()

	val, err := indexer.FromArgs(uuid)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(uuidBuf, val) {
		t.Fatalf("foo")
	}

	val, err = indexer.FromArgs(uuidBuf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(uuidBuf, val) {
		t.Fatalf("foo")
	}
}

func generateUUID() ([]byte, string) {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
	return buf, uuid
}

func TestCompoundIndex_FromObject(t *testing.T) {
	obj := testObj()
	indexer := &CompoundIndex{
		Indexes: []Indexer{
			&StringFieldIndex{"ID", false},
			&StringFieldIndex{"Foo", false},
			&StringFieldIndex{"Baz", false},
		},
		AllowMissing: false,
	}

	ok, val, err := indexer.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "my-cool-obj\x00Testing\x00yep\x00" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	missing := &CompoundIndex{
		Indexes: []Indexer{
			&StringFieldIndex{"ID", false},
			&StringFieldIndex{"Foo", true},
			&StringFieldIndex{"Empty", false},
		},
		AllowMissing: true,
	}
	ok, val, err = missing.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "my-cool-obj\x00testing\x00" {
		t.Fatalf("bad: %s", val)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	// Test when missing not allowed
	missing.AllowMissing = false
	ok, _, err = missing.FromObject(obj)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("should not be okay")
	}
}

func TestCompoundIndex_FromArgs(t *testing.T) {
	indexer := &CompoundIndex{
		Indexes: []Indexer{
			&StringFieldIndex{"ID", false},
			&StringFieldIndex{"Foo", false},
			&StringFieldIndex{"Baz", false},
		},
		AllowMissing: false,
	}
	_, err := indexer.FromArgs()
	if err == nil {
		t.Fatalf("should get err")
	}

	_, err = indexer.FromArgs(42, 42, 42)
	if err == nil {
		t.Fatalf("should get err")
	}

	val, err := indexer.FromArgs("foo", "bar", "baz")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(val) != "foo\x00bar\x00baz\x00" {
		t.Fatalf("bad: %s", val)
	}
}

func TestCompoundIndex_PrefixFromArgs(t *testing.T) {
	indexer := &CompoundIndex{
		Indexes: []Indexer{
			&UUIDFieldIndex{"ID"},
			&StringFieldIndex{"Foo", false},
			&StringFieldIndex{"Baz", false},
		},
		AllowMissing: false,
	}
	val, err := indexer.PrefixFromArgs()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(val) != 0 {
		t.Fatalf("bad: %s", val)
	}

	uuidBuf, uuid := generateUUID()
	val, err = indexer.PrefixFromArgs(uuid, "foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(val[:16], uuidBuf) {
		t.Fatalf("bad prefix")
	}
	if string(val[16:]) != "foo\x00" {
		t.Fatalf("bad: %s", val)
	}
}

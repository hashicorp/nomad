package testing

import (
	"fmt"
	"log"
)

// T is the interface that mimics the standard library *testing.T.
//
// In unit tests you can just pass a *testing.T struct. At runtime, outside
// of tests, you can pass in a RuntimeT struct from this package.
type T interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

// RuntimeT implements T and can be instantiated and run at runtime to
// mimic *testing.T behavior. Unlike *testing.T, this will simply panic
// for calls to Fatal. For calls to Error, you'll have to check the errors
// list to determine whether to exit yourself.
type RuntimeT struct {
	failed bool
}

func (t *RuntimeT) Error(args ...interface{}) {
	log.Println(fmt.Sprintln(args...))
	t.Fail()
}

func (t *RuntimeT) Errorf(format string, args ...interface{}) {
	log.Println(fmt.Sprintf(format, args...))
	t.Fail()
}

func (t *RuntimeT) Fatal(args ...interface{}) {
	log.Println(fmt.Sprintln(args...))
	t.FailNow()
}

func (t *RuntimeT) Fatalf(format string, args ...interface{}) {
	log.Println(fmt.Sprintf(format, args...))
	t.FailNow()
}

func (t *RuntimeT) Fail() {
	t.failed = true
}

func (t *RuntimeT) FailNow() {
	panic("testing.T failed, see logs for output (if any)")
}

func (t *RuntimeT) Failed() bool {
	return t.failed
}

func (t *RuntimeT) Log(args ...interface{}) {
	log.Println(fmt.Sprintln(args...))
}

func (t *RuntimeT) Logf(format string, args ...interface{}) {
	log.Println(fmt.Sprintf(format, args...))
}

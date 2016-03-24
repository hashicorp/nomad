package consul

import (
	"reflect"
	"testing"
	"time"
)

func TestCheckHeapOrder(t *testing.T) {
	h := NewConsulChecksHeap()

	c1 := ExecScriptCheck{id: "a"}
	c2 := ExecScriptCheck{id: "b"}
	c3 := ExecScriptCheck{id: "c"}

	lookup := map[Check]string{
		&c1: "c1",
		&c2: "c2",
		&c3: "c3",
	}

	h.Push(&c1, time.Time{})
	h.Push(&c2, time.Unix(10, 0))
	h.Push(&c3, time.Unix(11, 0))

	expected := []string{"c2", "c3", "c1"}
	var actual []string
	for i := 0; i < 3; i++ {
		cCheck := h.Pop()

		actual = append(actual, lookup[cCheck.check])
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Wrong ordering; got %v; want %v", actual, expected)
	}
}

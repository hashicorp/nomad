package stats

import (
	"testing"
)

func TestRingBuffInvalid(t *testing.T) {
	if _, err := NewRingBuff(0); err == nil {
		t.Fatalf("expected err")
	}
}

func TestRingBuffEnqueue(t *testing.T) {
	rb, err := NewRingBuff(3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rb.Enqueue(1)
	rb.Enqueue(2)
	rb.Enqueue(3)
	if val := rb.Peek(); val != 3 {
		t.Fatalf("expected: %v, actual: %v", 3, val)
	}

	rb.Enqueue(4)
	rb.Enqueue(5)
	if val := rb.Peek(); val != 5 {
		t.Fatalf("expected: %v, actual: %v", 5, val)
	}
}

func TestRingBuffValues(t *testing.T) {
	rb, err := NewRingBuff(3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rb.Enqueue(1)
	rb.Enqueue(2)
	rb.Enqueue(3)
	rb.Enqueue(4)

	expected := []interface{}{2, 3, 4}
	if !sliceEq(expected, rb.Values()) {
		t.Fatalf("expected: %v, actual: %v", expected, rb.Values())
	}

	rb.Enqueue(5)
	expected = []interface{}{3, 4, 5}
	if !sliceEq(expected, rb.Values()) {
		t.Fatalf("expected: %v, actual: %v", expected, rb.Values())
	}

	rb.Enqueue(6)
	expected = []interface{}{4, 5, 6}
	if !sliceEq(expected, rb.Values()) {
		t.Fatalf("expected: %v, actual: %v", expected, rb.Values())
	}

}

func sliceEq(slice1, slice2 []interface{}) bool {

	if slice1 == nil && slice2 == nil {
		return true
	}

	if slice1 == nil || slice2 == nil {
		return false
	}

	if len(slice1) != len(slice2) {
		return false
	}

	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

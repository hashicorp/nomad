package structs

import "testing"

func TestBitmap(t *testing.T) {
	// Check invalid sizes
	_, err := NewBitmap(0)
	if err == nil {
		t.Fatalf("bad")
	}
	_, err = NewBitmap(7)
	if err == nil {
		t.Fatalf("bad")
	}

	// Create a normal bitmap
	b, err := NewBitmap(256)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Set a few bits
	b.Set(0)
	b.Set(255)

	// Verify the bytes
	if b[0] == 0 {
		t.Fatalf("bad")
	}
	if !b.Check(0) {
		t.Fatalf("bad")
	}

	// Verify the bytes
	if b[len(b)-1] == 0 {
		t.Fatalf("bad")
	}
	if !b.Check(255) {
		t.Fatalf("bad")
	}

	// All other bits should be unset
	for i := 1; i < 255; i++ {
		if b.Check(uint(i)) {
			t.Fatalf("bad")
		}
	}

	// Clear
	b.Clear()

	// All bits should be unset
	for i := 0; i < 256; i++ {
		if b.Check(uint(i)) {
			t.Fatalf("bad")
		}
	}
}

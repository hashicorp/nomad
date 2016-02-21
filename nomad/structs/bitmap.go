package structs

import "fmt"

// Bitmap is a simple uncompressed bitmap
type Bitmap []byte

// NewBitmap returns a bitmap with up to size indexes
func NewBitmap(size int) (Bitmap, error) {
	if size <= 0 {
		return nil, fmt.Errorf("bitmap must be positive size")
	}
	if size&7 != 0 {
		return nil, fmt.Errorf("bitmap must be byte aligned")
	}
	b := make([]byte, size>>3)
	return Bitmap(b), nil
}

// Set is used to set the given index of the bitmap
func (b Bitmap) Set(idx uint) {
	bucket := idx >> 3
	mask := byte(1 << (idx & 7))
	b[bucket] |= mask
}

// Check is used to check the given index of the bitmap
func (b Bitmap) Check(idx uint) bool {
	bucket := idx >> 3
	mask := byte(1 << (idx & 7))
	return (b[bucket] & mask) != 0
}

// Clear is used to efficiently clear the bitmap
func (b Bitmap) Clear() {
	for i := range b {
		b[i] = 0
	}
}

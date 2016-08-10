package structs

import "fmt"

// Bitmap is a simple uncompressed bitmap
type Bitmap []byte

// NewBitmap returns a bitmap with up to size indexes
func NewBitmap(size uint) (Bitmap, error) {
	if size == 0 {
		return nil, fmt.Errorf("bitmap must be positive size")
	}
	if size&7 != 0 {
		return nil, fmt.Errorf("bitmap must be byte aligned")
	}
	b := make([]byte, size>>3)
	return Bitmap(b), nil
}

// Copy returns a copy of the Bitmap
func (b Bitmap) Copy() (Bitmap, error) {
	if b == nil {
		return nil, fmt.Errorf("can't copy nil Bitmap")
	}

	nb, err := NewBitmap(b.Size())
	if err != nil {
		return nil, err
	}

	s := b.Size()
	for i := uint(0); i < s; i++ {
		if b.Check(i) {
			nb.Set(i)
		}
	}

	return nb, nil
}

// Size returns the size of the bitmap
func (b Bitmap) Size() uint {
	return uint(len(b) << 3)
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

// IndexesFrom returns the indexes in which the values are either set or unset based
// on the passed parameter starting from the passed index
func (b Bitmap) IndexesFrom(set bool, from uint) []int {
	var indexes []int
	s := b.Size()
	for i := from; i < s; i++ {
		c := b.Check(i)
		if c && set || !c && !set {
			indexes = append(indexes, int(i))
		}
	}

	return indexes
}

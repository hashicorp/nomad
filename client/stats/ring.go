package stats

import (
	"fmt"
)

var (
	defaultCap = 60
)

type RingBuff struct {
	head int
	buff []interface{}
}

func NewRingBuff(capacity int) (*RingBuff, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("can not create a ring buffer with capacity: %v", capacity)
	}
	return &RingBuff{buff: make([]interface{}, capacity), head: -1}, nil
}

func (r *RingBuff) Enqueue(value interface{}) {
	r.head += 1
	if r.head == len(r.buff) {
		r.head = 0
	}
	r.buff[r.head] = value
}

func (r *RingBuff) Peek() interface{} {
	return r.buff[r.head]
}

func (r *RingBuff) Values() []interface{} {
	if r.head == len(r.buff)-1 {
		vals := make([]interface{}, len(r.buff))
		copy(vals, r.buff)
		return vals
	}

	slice1 := r.buff[r.head+1:]
	slice2 := r.buff[:r.head+1]
	return append(slice1, slice2...)
}

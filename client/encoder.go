package client

import (
	"sync"
)

type encoder interface {
	Encode(v interface{}) error
	MustEncode(v interface{})
}

type syncEncoder struct {
	impl encoder
	lock sync.Mutex
}

func (e *syncEncoder) Encode(v interface{}) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.impl.Encode(v)
}

func (e *syncEncoder) MustEncode(v interface{}) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.impl.MustEncode(v)
}

func newSyncEncoder(e encoder) encoder {
	return &syncEncoder{impl: e}
}

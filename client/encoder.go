package client

import (
	"sync"
)

type encoder interface {
	Encode(v interface{}) error
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

func newSyncEncoder(e encoder) encoder {
	return &syncEncoder{impl: e}
}

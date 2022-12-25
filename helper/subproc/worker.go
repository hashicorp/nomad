package subproc

import (
	"time"
)

// Func is the function to be run as a sub-process.
//
// The return value is a Unix process exit code.
type Func func() int

type Option func(w *Worker)

func Start(w *Worker) {

}

type Worker struct {
	deadline time.Time
}

func New(name string, f Func, opts ...Option) *Worker {
	w := new(Worker)
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func Timeout(duration time.Duration) Option {
	return func(w *Worker) {
		w.deadline = time.Now().Add(duration)
	}
}

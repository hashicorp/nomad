package circbufwriter

import (
	"io"
	"sync"
	"time"

	"github.com/armon/circbuf"
)

type circbufWriter struct {
	// circle buffer for data to write
	buf *circbuf.Buffer

	// error to return from the writer
	err error

	// bufLock syncronizes access to err and buf
	bufLock sync.Mutex

	// wrapped writer
	wr io.Writer

	// signals to flush the buffer
	flushCh chan struct{}
}

// New created a circle buffered writer that wraps the given writer. The
// bufferSize is the amount of data that will be stored in memory before
// overwriting.
func New(w io.Writer, bufferSize int64) io.WriteCloser {
	buf, _ := circbuf.NewBuffer(bufferSize)
	c := &circbufWriter{
		buf:     buf,
		wr:      w,
		flushCh: make(chan struct{}, 1),
	}
	go c.flushLoop()
	return c
}

// Write will write the data to the buffer and attempt to flush the buffer to
// the wrapped writer. If the wrapped writer blocks on write, subsequent write
// will be written to the circle buffer.
func (c *circbufWriter) Write(p []byte) (nn int, err error) {
	// If the last write returned an error, return it here. Note there is a
	// small chance of missing an error if multiple writes occure at the same
	// time where the last write nils out the error before it can be returned
	// here.
	c.bufLock.Lock()
	defer c.bufLock.Unlock()
	if c.err != nil {
		return nn, c.err
	}

	// Write to the buffer
	nn, err = c.buf.Write(p)

	// Signal to flush the buffer
	select {
	case c.flushCh <- struct{}{}:
	default:
		// flush is blocked
	}
	return nn, err
}

func (c *circbufWriter) Close() error {
	// Guard against double closing channel
	select {
	case <-c.flushCh:
	default:
		close(c.flushCh)
	}

	// if the last write errored, it will return here
	c.bufLock.Lock()
	defer c.bufLock.Unlock()
	return c.err
}

func (c *circbufWriter) flushLoop() {
	// Check buffer every 100ms in case a flush from Write was missed
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		var err error
		select {
		case _, ok := <-c.flushCh:
			if !ok {
				// Close called, exiting loop
				return
			}
			err = c.flush()
		case <-ticker.C:
			err = c.flush()
		}

		c.bufLock.Lock()
		c.err = err
		c.bufLock.Unlock()
	}
}

func (c *circbufWriter) flush() error {
	c.bufLock.Lock()
	b := c.buf.Bytes()
	c.buf.Reset()
	c.bufLock.Unlock()

	var err error
	if len(b) > 0 {
		_, err = c.wr.Write(b)
	}
	return err
}

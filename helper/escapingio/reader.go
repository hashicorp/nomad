package escapingio

import (
	"io"
)

// Handler is a callback for handling an escaped char.  Reader would skip
// the escape char and passed char if returns true; otherwise, it preserves them
// in output
type Handler func(c byte) bool

// NewReader returns a reader that escapes the c character (following new lines),
// in the same manner OpenSSH handling, which defaults to `~`.
//
// For illustrative purposes, we use `~` in documentation as a shorthand for escaping character.
//
// If following a new line, reader sees:
//  * `~~`, only one is emitted
//  * `~.` (or any character), the handler is invoked with the character.
//     If handler returns true, `~.` will be skipped; otherwise, it's propagated.
//  * `~` and it's the last character in stream, it's propagated
//
// Appearances of `~` when not followed by a new line is propagated unmodified.
func NewReader(r io.Reader, c byte, h Handler) io.Reader {
	return &reader{
		impl:       r,
		escapeChar: c,
		state:      sLookEscapeChar,
		handler:    h,
	}
}

// lookState represents the state of reader for what character of `\n~.` sequence
// reader is looking for
type lookState int

const (
	// sLookNewLine indicates that reader is looking for new line
	sLookNewLine lookState = iota

	// sLookEscapeChar indicates that reader is looking for ~
	sLookEscapeChar

	// sLookChar indicates that reader just read `~` is waiting for next character
	// before acting
	sLookChar
)

// to ease comments, i'll assume escape character to be `~`
type reader struct {
	impl       io.Reader
	escapeChar uint8
	handler    Handler

	state lookState

	// unread is a buffered character for next read if not-nil
	unread *byte
}

func (r *reader) Read(buf []byte) (int, error) {
START:
	var n int
	var err error

	if r.unread != nil {
		// try to return the unread character immediately
		// without trying to block for another read
		buf[0] = *r.unread
		n = 1
		r.unread = nil
	} else {
		n, err = r.impl.Read(buf)
	}

	// when we get to the end, check if we have any unprocessed \n~
	if n == 0 && err != nil {
		if r.state == sLookChar && err != nil {
			buf[0] = r.escapeChar
			n = 1
		}
		return n, err
	}

	// inspect the state at beginning of read
	if r.state == sLookChar {
		r.state = sLookNewLine

		// escape character hasn't been emitted yet
		if buf[0] == r.escapeChar {
			// earlier ~ was sallowed already, so leave this as is
		} else if handled := r.handler(buf[0]); handled {
			// need to drop a single letter
			copy(buf, buf[1:n])
			n--
		} else {
			// we need to re-introduce ~ with rest of body
			// but be mindful if reintroducing ~ causes buffer to overflow
			if n == len(buf) {
				// in which case, save it for next read
				c := buf[n-1]
				r.unread = &c
				copy(buf[1:], buf[:n])
				buf[0] = r.escapeChar
			} else {
				copy(buf[1:], buf[:n])
				buf[0] = r.escapeChar
				n++
			}
		}
	}

	n = r.processBuffer(buf, n)
	if n == 0 && err == nil {
		goto START
	}

	return n, err
}

// handles escaped character inside body of read buf.
func (r *reader) processBuffer(buf []byte, read int) int {
	b := 0

	for b < read {

		c := buf[b]
		if r.state == sLookEscapeChar && r.escapeChar == c {
			r.state = sLookEscapeChar

			// are we at the end of read; wait for next read
			if b == read-1 {
				read--
				r.state = sLookChar
				return read
			}

			// otherwise peek at next
			nc := buf[b+1]
			if nc == r.escapeChar {
				// repeated ~, only emit one - skip one character
				copy(buf[b:], buf[b+1:read])
				read--
				b++
				continue
			} else if handled := r.handler(nc); handled {
				// need to drop both ~ and letter
				copy(buf[b:], buf[b+2:read])
				read -= 2
				continue
			} else {
				// need to pass output unmodified with ~ and letter
			}
		} else if c == '\n' || c == '\r' {
			r.state = sLookEscapeChar
		} else {
			r.state = sLookNewLine
		}
		b++
	}

	return read
}

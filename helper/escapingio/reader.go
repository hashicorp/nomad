// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package escapingio

import (
	"bufio"
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
//   - `~~`, only one is emitted
//   - `~.` (or any character), the handler is invoked with the character.
//     If handler returns true, `~.` will be skipped; otherwise, it's propagated.
//   - `~` and it's the last character in stream, it's propagated
//
// Appearances of `~` when not preceded by a new line are propagated unmodified.
func NewReader(r io.Reader, c byte, h Handler) io.Reader {
	pr, pw := io.Pipe()
	reader := &reader{
		impl:       r,
		escapeChar: c,
		handler:    h,
		pr:         pr,
		pw:         pw,
	}
	go reader.pipe()
	return reader
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

	// buffers
	pw *io.PipeWriter
	pr *io.PipeReader
}

func (r *reader) Read(buf []byte) (int, error) {
	return r.pr.Read(buf)
}

func (r *reader) pipe() {
	rb := make([]byte, 4096)
	bw := bufio.NewWriter(r.pw)

	state := sLookEscapeChar

	for {
		n, err := r.impl.Read(rb)

		if n > 0 {
			state = r.processBuf(bw, rb, n, state)
			bw.Flush()
			if state == sLookChar {
				// terminated with ~ - let's read one more character
				n, err = r.impl.Read(rb[:1])
				if n == 1 {
					state = sLookNewLine
					if rb[0] == r.escapeChar {
						// only emit escape character once
						bw.WriteByte(rb[0])
						bw.Flush()
					} else if r.handler(rb[0]) {
						// skip if handled
					} else {
						bw.WriteByte(r.escapeChar)
						bw.WriteByte(rb[0])
						bw.Flush()
						if rb[0] == '\n' || rb[0] == '\r' {
							state = sLookEscapeChar
						}
					}
				}
			}
		}

		if err != nil {
			// write ~ if it's the last thing
			if state == sLookChar {
				bw.WriteByte(r.escapeChar)
			}
			bw.Flush()
			r.pw.CloseWithError(err)
			break
		}
	}
}

// processBuf process buffer and emits all output to writer
// if the last part of buffer is a new line followed by sequnce, it writes
// all output until the new line and returns sLookChar
func (r *reader) processBuf(bw io.Writer, buf []byte, n int, s lookState) lookState {
	i := 0

	wi := 0

START:
	if s == sLookEscapeChar && buf[i] == r.escapeChar {
		if i+1 >= n {
			// buf terminates with ~ - write all before
			bw.Write(buf[wi:i])
			return sLookChar
		}

		nc := buf[i+1]
		if nc == r.escapeChar {
			// skip one escape char
			bw.Write(buf[wi:i])
			i++
			wi = i
		} else if r.handler(nc) {
			// skip both characters
			bw.Write(buf[wi:i])
			i = i + 2
			wi = i
		} else if nc == '\n' || nc == '\r' {
			i = i + 2
			s = sLookEscapeChar
			goto START
		} else {
			i = i + 2
			// need to write everything keep going
		}
	}

	// search until we get \n~, or buf terminates
	for {
		if i >= n {
			// got to end without new line, write and return
			bw.Write(buf[wi:n])
			return sLookNewLine
		}

		if buf[i] == '\n' || buf[i] == '\r' {
			// buf terminated at new line
			if i+1 >= n {
				bw.Write(buf[wi:n])
				return sLookEscapeChar
			}

			// peek to see escape character go back to START if so
			if buf[i+1] == r.escapeChar {
				s = sLookEscapeChar
				i++
				goto START
			}
		}

		i++
	}
}

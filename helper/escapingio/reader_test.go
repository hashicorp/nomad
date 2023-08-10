// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package escapingio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"testing/quick"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapingReader_Static(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		escaped  string
	}{
		{"hello", "hello", ""},
		{"he\nllo", "he\nllo", ""},
		{"he~.lo", "he~.lo", ""},
		{"he\n~.rest", "he\nrest", "."},
		{"he\n~.r\n~.est", "he\nr\nest", ".."},
		{"he\n~~r\n~~est", "he\n~r\n~est", ""},
		{"he\n~~r\n~.est", "he\n~r\nest", "."},
		{"he\nr~~est", "he\nr~~est", ""},
		{"he\nr\n~qest", "he\nr\n~qest", "q"},
		{"he\nr\r~qe\r~.st", "he\nr\r~qe\rst", "q."},
		{"~q", "~q", "q"},
		{"~.", "", "."},
		{"m~.", "m~.", ""},
		{"\n~.", "\n", "."},
		{"~", "~", ""},
		{"\r~.", "\r", "."},
		{"b\n~\n~.q", "b\n~\nq", "."},
	}

	for _, c := range cases {
		t.Run("validate naive implementation", func(t *testing.T) {
			h := &testHandler{}

			processed := naiveEscapeCharacters(c.input, '~', h.handler)
			require.Equal(t, c.expected, processed)
			require.Equal(t, c.escaped, h.escaped())
		})

		t.Run("chunks at a time: "+c.input, func(t *testing.T) {
			var found bytes.Buffer

			input := strings.NewReader(c.input)

			h := &testHandler{}

			filter := NewReader(input, '~', h.handler)

			_, err := io.Copy(&found, filter)
			require.NoError(t, err)

			require.Equal(t, c.expected, found.String())
			require.Equal(t, c.escaped, h.escaped())
		})

		t.Run("1 byte at a time: "+c.input, func(t *testing.T) {
			var found bytes.Buffer

			input := iotest.OneByteReader(strings.NewReader(c.input))

			h := &testHandler{}

			filter := NewReader(input, '~', h.handler)
			_, err := io.Copy(&found, filter)
			require.NoError(t, err)

			require.Equal(t, c.expected, found.String())
			require.Equal(t, c.escaped, h.escaped())
		})

		t.Run("without reading: "+c.input, func(t *testing.T) {
			input := strings.NewReader(c.input)

			h := &testHandler{}

			filter := NewReader(input, '~', h.handler)

			// don't read to mimic a stalled reader
			_ = filter

			assertEventually(t, func() (bool, error) {
				escaped := h.escaped()
				if c.escaped == escaped {
					return true, nil
				}

				return false, fmt.Errorf("expected %v but found %v", c.escaped, escaped)
			})
		})
	}
}

// TestEscapingReader_EmitsPartialReads should emit partial results
// if next character is not read
func TestEscapingReader_FlushesPartialReads(t *testing.T) {
	pr, pw := io.Pipe()

	h := &testHandler{}
	filter := NewReader(pr, '~', h.handler)

	var lock sync.Mutex
	var read bytes.Buffer

	// helper for asserting reads
	requireRead := func(expected *bytes.Buffer) {
		readSoFar := ""

		start := time.Now()
		for time.Since(start) < 2*time.Second {
			lock.Lock()
			readSoFar = read.String()
			lock.Unlock()

			if readSoFar == expected.String() {
				break
			}

			time.Sleep(50 * time.Millisecond)
		}

		require.Equal(t, expected.String(), readSoFar, "timed out without output")
	}

	var rerr error
	var wg sync.WaitGroup
	wg.Add(1)

	// goroutine for reading partial data
	go func() {
		defer wg.Done()

		buf := make([]byte, 1024)
		for {
			n, err := filter.Read(buf)
			lock.Lock()
			read.Write(buf[:n])
			lock.Unlock()

			if err != nil {
				rerr = err
				break
			}
		}
	}()

	expected := &bytes.Buffer{}

	// test basic start and no new lines
	pw.Write([]byte("first data"))
	expected.WriteString("first data")
	requireRead(expected)
	require.Equal(t, "", h.escaped())

	// test ~. appearing in middle of line but stop at new line
	pw.Write([]byte("~.inmiddleappears\n"))
	expected.WriteString("~.inmiddleappears\n")
	requireRead(expected)
	require.Equal(t, "", h.escaped())

	// from here on we test \n~ at boundary

	// ~~ after new line; and stop at \n~
	pw.Write([]byte("~~second line\n~"))
	expected.WriteString("~second line\n")
	requireRead(expected)
	require.Equal(t, "", h.escaped())

	// . to be skipped; stop at \n~ again
	pw.Write([]byte(".third line\n~"))
	expected.WriteString("third line\n")
	requireRead(expected)
	require.Equal(t, ".", h.escaped())

	// q to be emitted; stop at \n
	pw.Write([]byte("qfourth line\n"))
	expected.WriteString("~qfourth line\n")
	requireRead(expected)
	require.Equal(t, ".q", h.escaped())

	// ~. to be skipped; stop at \n~
	pw.Write([]byte("~.fifth line\n~"))
	expected.WriteString("fifth line\n")
	requireRead(expected)
	require.Equal(t, ".q.", h.escaped())

	// ~ alone after \n~ - should be emitted
	pw.Write([]byte("~"))
	expected.WriteString("~")
	requireRead(expected)
	require.Equal(t, ".q.", h.escaped())

	// rest of line ending with \n~
	pw.Write([]byte("rest of line\n~"))
	expected.WriteString("rest of line\n")
	requireRead(expected)
	require.Equal(t, ".q.", h.escaped())

	// m alone after \n~ - should be emitted with ~
	pw.Write([]byte("m"))
	expected.WriteString("~m")
	requireRead(expected)
	require.Equal(t, ".q.m", h.escaped())

	// rest of line and end with \n
	pw.Write([]byte("onemore line\n"))
	expected.WriteString("onemore line\n")
	requireRead(expected)
	require.Equal(t, ".q.m", h.escaped())

	// ~q to be emitted stop at \n~; last charcater
	pw.Write([]byte("~qlast line\n~"))
	expected.WriteString("~qlast line\n")
	requireRead(expected)
	require.Equal(t, ".q.mq", h.escaped())

	// last ~ gets emitted and we preserve error
	eerr := errors.New("my custom error")
	pw.CloseWithError(eerr)
	expected.WriteString("~")
	requireRead(expected)
	require.Equal(t, ".q.mq", h.escaped())

	wg.Wait()
	require.Error(t, rerr)
	require.Equal(t, eerr, rerr)
}

func TestEscapingReader_Generated_EquivalentToNaive(t *testing.T) {
	f := func(v readingInput) bool {
		return checkEquivalenceToNaive(t, string(v))
	}

	require.NoError(t, quick.Check(f, &quick.Config{
		MaxCountScale: 200,
	}))
}

// testHandler is a conveneient struct for finding "escaped" ascii letters
// in escaping reader.
// We avoid complicated unicode characters that may cross byte boundary
type testHandler struct {
	l      sync.Mutex
	result string
}

// handler is method to be passed to escaping io reader
func (t *testHandler) handler(c byte) bool {
	rc := rune(c)
	simple := unicode.IsLetter(rc) ||
		unicode.IsDigit(rc) ||
		unicode.IsPunct(rc) ||
		unicode.IsSymbol(rc)

	if simple {
		t.l.Lock()
		t.result += string([]byte{c})
		t.l.Unlock()
	}
	return c == '.'
}

// escaped returns all seen escaped characters so far
func (t *testHandler) escaped() string {
	t.l.Lock()
	defer t.l.Unlock()

	return t.result
}

// checkEquivalence returns true if parsing input with naive implementation
// is equivalent to our reader
func checkEquivalenceToNaive(t *testing.T, input string) bool {
	nh := &testHandler{}
	expected := naiveEscapeCharacters(input, '~', nh.handler)

	foundH := &testHandler{}

	var inputReader io.Reader = bytes.NewBufferString(input)
	inputReader = &arbtiraryReader{
		buf:         inputReader.(*bytes.Buffer),
		maxReadOnce: 10,
	}
	filter := NewReader(inputReader, '~', foundH.handler)
	var found bytes.Buffer
	_, err := io.Copy(&found, filter)
	if err != nil {
		t.Logf("unexpected error while reading: %v", err)
		return false
	}

	if nh.escaped() == foundH.escaped() && expected == found.String() {
		return true
	}

	t.Logf("escaped differed=%v expected=%v found=%v", nh.escaped() != foundH.escaped(), nh.escaped(), foundH.escaped())
	t.Logf("read  differed=%v expected=%s found=%v", expected != found.String(), expected, found.String())
	return false

}

func TestEscapingReader_Generated_EquivalentToReadOnce(t *testing.T) {
	f := func(v readingInput) bool {
		return checkEquivalenceToNaive(t, string(v))
	}

	require.NoError(t, quick.Check(f, &quick.Config{
		MaxCountScale: 200,
	}))
}

// checkEquivalenceToReadOnce returns true if parsing input in a single
// read matches multiple reads
func checkEquivalenceToReadOnce(t *testing.T, input string) bool {
	nh := &testHandler{}
	var expected bytes.Buffer

	// getting expected value from read all at once
	{
		buf := make([]byte, len(input)+5)
		inputReader := NewReader(bytes.NewBufferString(input), '~', nh.handler)
		_, err := io.CopyBuffer(&expected, inputReader, buf)
		if err != nil {
			t.Logf("unexpected error while reading: %v", err)
			return false
		}
	}

	foundH := &testHandler{}
	var found bytes.Buffer

	// getting found by using arbitrary reader
	{
		inputReader := &arbtiraryReader{
			buf:         bytes.NewBufferString(input),
			maxReadOnce: 10,
		}
		filter := NewReader(inputReader, '~', foundH.handler)
		_, err := io.Copy(&found, filter)
		if err != nil {
			t.Logf("unexpected error while reading: %v", err)
			return false
		}
	}

	if nh.escaped() == foundH.escaped() && expected.String() == found.String() {
		return true
	}

	t.Logf("escaped differed=%v expected=%v found=%v", nh.escaped() != foundH.escaped(), nh.escaped(), foundH.escaped())
	t.Logf("read  differed=%v expected=%s found=%v", expected.String() != found.String(), expected.String(), found.String())
	return false

}

// readingInput is a string with some quick generation capability to
// inject some \n, \n~., \n~q in text
type readingInput string

func (i readingInput) Generate(rand *rand.Rand, size int) reflect.Value {
	v, ok := quick.Value(reflect.TypeOf(""), rand)
	if !ok {
		panic("couldn't generate a string")
	}

	// inject some terminals
	var b bytes.Buffer
	injectProbabilistically := func() {
		p := rand.Float32()
		if p < 0.05 {
			b.WriteString("\n~.")
		} else if p < 0.10 {
			b.WriteString("\n~q")
		} else if p < 0.15 {
			b.WriteString("\n")
		} else if p < 0.2 {
			b.WriteString("~")
		} else if p < 0.25 {
			b.WriteString("~~")
		}
	}

	for _, c := range v.String() {
		injectProbabilistically()
		b.WriteRune(c)
	}

	injectProbabilistically()

	return reflect.ValueOf(readingInput(b.String()))
}

// naiveEscapeCharacters is a simplified implementation that operates
// on entire unchunked string.  Uses regexp implementation.
//
// It differs from the other implementation in handling unicode characters
// proceeding `\n~`
func naiveEscapeCharacters(input string, escapeChar byte, h Handler) string {
	reg := regexp.MustCompile(fmt.Sprintf("(\n|\r)%c.", escapeChar))

	// check first appearances
	if len(input) > 1 && input[0] == escapeChar {
		if input[1] == escapeChar {
			input = input[1:]
		} else if h(input[1]) {
			input = input[2:]
		} else {
			// we are good
		}

	}

	return reg.ReplaceAllStringFunc(input, func(match string) string {
		// match can be more than three bytes because of unicode
		if len(match) < 3 {
			panic(fmt.Errorf("match is less than characters: %d %s", len(match), match))
		}

		c := match[2]

		// ignore some unicode partial codes
		ltr := len(match) > 3 ||
			('a' <= c && c <= 'z') ||
			('A' <= c && c <= 'Z') ||
			('0' <= c && c <= '9') ||
			(c == '~' || c == '.' || c == escapeChar)

		if c == escapeChar {
			return match[:2]
		} else if ltr && h(c) {
			return match[:1]
		} else {
			return match
		}
	})
}

// arbitraryReader is a reader that reads arbitrary length at a time
// to simulate input being read in chunks.
type arbtiraryReader struct {
	buf         *bytes.Buffer
	maxReadOnce int
}

func (r *arbtiraryReader) Read(buf []byte) (int, error) {
	l := r.buf.Len()
	if l == 0 || l == 1 {
		return r.buf.Read(buf)
	}

	if l > r.maxReadOnce {
		l = r.maxReadOnce
	}
	if l != 1 {
		l = rand.Intn(l-1) + 1
	}
	if l > len(buf) {
		l = len(buf)
	}

	return r.buf.Read(buf[:l])
}

func assertEventually(t *testing.T, testFn func() (bool, error)) {
	start := time.Now()
	var err error
	var b bool
	for {
		if time.Since(start) > 2*time.Second {
			assert.Fail(t, "timed out", "error: %v", err)
		}

		b, err = testFn()
		if b {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}
}

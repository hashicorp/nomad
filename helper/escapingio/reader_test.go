package escapingio

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
	"testing/quick"
	"unicode"

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
	}

	for _, c := range cases {
		t.Run("sanity check naive implementation", func(t *testing.T) {
			foundEscaped := ""
			h := testHandler(&foundEscaped)

			processed := naiveEscapeCharacters(c.input, '~', h)
			require.Equal(t, c.expected, processed)
			require.Equal(t, c.escaped, foundEscaped)
		})

		t.Run("chunks at a time: "+c.input, func(t *testing.T) {
			var found bytes.Buffer

			input := strings.NewReader(c.input)

			foundEscaped := ""
			h := testHandler(&foundEscaped)

			filter := NewReader(input, '~', h)

			_, err := io.Copy(&found, filter)
			require.NoError(t, err)

			require.Equal(t, c.expected, found.String())
			require.Equal(t, c.escaped, foundEscaped)
		})

		t.Run("1 byte at a time: "+c.input, func(t *testing.T) {
			var found bytes.Buffer

			input := iotest.OneByteReader(strings.NewReader(c.input))

			foundEscaped := ""
			h := testHandler(&foundEscaped)

			filter := NewReader(input, '~', h)
			_, err := io.Copy(&found, filter)
			require.NoError(t, err)

			require.Equal(t, c.expected, found.String())
			require.Equal(t, c.escaped, foundEscaped)
		})
	}
}

func TestEscapingReader_Generated_EquivalentToNaive(t *testing.T) {
	f := func(v readingInput) bool {
		return checkEquivalenceToNaive(t, string(v))
	}

	require.NoError(t, quick.Check(f, &quick.Config{
		MaxCountScale: 200,
	}))
}

// testHandler returns a handler that stores all basic ascii letters in result
// reference.  We avoid complicated unicode characters that may cross
// byte boundary
func testHandler(result *string) Handler {
	return func(c byte) bool {
		rc := rune(c)
		simple := unicode.IsLetter(rc) ||
			unicode.IsDigit(rc) ||
			unicode.IsPunct(rc) ||
			unicode.IsSymbol(rc)

		if simple {
			*result += string([]byte{c})
		}
		return c == '.'
	}
}

// checkEquivalence returns true if parsing input with naive implementation
// is equivalent to our reader
func checkEquivalenceToNaive(t *testing.T, input string) bool {
	nfe := ""
	nh := testHandler(&nfe)
	expected := naiveEscapeCharacters(input, '~', nh)

	foundEscaped := ""
	h := testHandler(&foundEscaped)

	var inputReader io.Reader = bytes.NewBufferString(input)
	inputReader = &arbtiraryReader{
		buf:         inputReader.(*bytes.Buffer),
		maxReadOnce: 10,
	}
	filter := NewReader(inputReader, '~', h)
	var found bytes.Buffer
	_, err := io.Copy(&found, filter)
	if err != nil {
		t.Logf("unexpected error while reading: %v", err)
		return false
	}

	if nfe == foundEscaped && expected == found.String() {
		return true
	}

	t.Logf("escaped differed=%v expected=%v found=%v", nfe != foundEscaped, nfe, foundEscaped)
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
	nfe := ""
	var expected bytes.Buffer

	// getting expected value from read all at once
	{
		h := testHandler(&nfe)

		buf := make([]byte, len(input)+5)
		inputReader := NewReader(bytes.NewBufferString(input), '~', h)
		_, err := io.CopyBuffer(&expected, inputReader, buf)
		if err != nil {
			t.Logf("unexpected error while reading: %v", err)
			return false
		}
	}

	foundEscaped := ""
	var found bytes.Buffer

	// getting found by using arbitrary reader
	{
		h := testHandler(&foundEscaped)

		inputReader := &arbtiraryReader{
			buf:         bytes.NewBufferString(input),
			maxReadOnce: 10,
		}
		filter := NewReader(inputReader, '~', h)
		_, err := io.Copy(&found, filter)
		if err != nil {
			t.Logf("unexpected error while reading: %v", err)
			return false
		}
	}

	if nfe == foundEscaped && expected.String() == found.String() {
		return true
	}

	t.Logf("escaped differed=%v expected=%v found=%v", nfe != foundEscaped, nfe, foundEscaped)
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
		if len(match) != 3 {
			panic(fmt.Errorf("match isn't 3 characters: %s", match))
		}

		c := match[2]

		// ignore some unicode partial codes
		ltr := ('a' <= c && c <= 'z') ||
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

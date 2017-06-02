// Package envparse is a minimal environment variable parser. It handles empty
// lines, comments, single quotes, double quotes, and JSON escape sequences.
//
// Non-empty or comment lines should be of the form:
//
//	KEY=value
//
// While extraneous characters are discouraged, an "export" prefix, preceeding
// whitespace, and trailing whitespace are all removed:
//
//	KEY = This is ok! # Parses to {"KEY": "This is ok!"}
//	KEY2= Also ok.    # Parses to {"KEY2": "Also ok."}
//	export FOO=bar    # Parses to {"FOO": "bar"}
package envparse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	ErrMissingSeparator = fmt.Errorf("missing =")
	ErrEmptyKey         = fmt.Errorf("empty key")
	ErrUnmatchedDouble  = fmt.Errorf(`unmatched "`)
	ErrUnmatchedSingle  = fmt.Errorf("unmatched '")
	ErrIncompleteEscape = fmt.Errorf("incomplete escape sequence")
	ErrIncompleteHex    = fmt.Errorf("incomplete hex sequence")
	ErrIncompleteSur    = fmt.Errorf("incomplete Unicode surrogate pair")
	ErrMultibyteEscape  = fmt.Errorf("multibyte characters disallowed in escape sequences")
)

// ParseError is returned whenever the Parse function encounters an error. It
// includes the line number and underlying error.
type ParseError struct {
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("error on line %d: %v", e.Line, e.Err)
	}
	return fmt.Sprintf("error reading: %v", e.Err)
}

func parseError(line int, err error) error {
	return &ParseError{
		Line: line,
		Err:  err,
	}
}

// Parse environment variables from an io.Reader into a map or return a
// ParseError.
func Parse(r io.Reader) (map[string]string, error) {
	env := make(map[string]string)
	scanner := bufio.NewScanner(r)

	// Track line number
	i := 0

	// Main scan loop
	for scanner.Scan() {
		i++
		k, v, err := parseLine(scanner.Bytes())
		if err != nil {
			return nil, parseError(i, err)
		}

		// Skip blank lines
		if len(k) > 0 {
			env[string(k)] = string(v)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, parseError(i, err)
	}
	return env, nil
}

const (
	normalMode  = iota
	doubleQuote = iota
	singleQuote = iota
	escapeMode  = iota
	unicodeMode = iota
)

var (
	empty        = []byte{}
	separator    = []byte{'='}
	exportPrefix = []byte("export ")
)

// parseLine parses the given line into a key and value or error.
//
// Empty lines are returned as zero length slices
func parseLine(ln []byte) ([]byte, []byte, error) {
	ln = bytes.TrimSpace(ln)
	if len(ln) == 0 || ln[0] == '#' {
		return empty, empty, nil
	}

	parts := bytes.SplitN(ln, separator, 2)
	if len(parts) != 2 {
		return nil, nil, ErrMissingSeparator
	}

	// Trim whitespace
	key, value := bytes.TrimSpace(parts[0]), bytes.TrimSpace(parts[1])

	// Ensure key is of the form [A-Za-z][A-Za-z0-9_]? with an optional
	// leading 'export ', but only trim leading export if there's another
	// key name.
	if len(key) > len(exportPrefix) {
		key = bytes.TrimPrefix(key, exportPrefix)
	}
	if len(key) == 0 {
		return nil, nil, ErrEmptyKey
	}
	if key[0] < 'A' {
		return nil, nil, fmt.Errorf("key must start with [A-Za-z_] but found %q", key[0])
	}
	if key[0] > 'Z' && key[0] < 'a' && key[0] != '_' {
		return nil, nil, fmt.Errorf("key must start with [A-Za-z_] but found %q", key[0])
	}
	if key[0] > 'z' {
		return nil, nil, fmt.Errorf("key must start with [A-Za-z_] but found %q", key[0])
	}

	for _, v := range key[1:] {
		switch {
		case v == '_':
		case v >= 'A' && v <= 'Z':
		case v >= 'a' && v <= 'z':
		case v >= '0' && v <= '9':
		default:
			return nil, nil, fmt.Errorf("key characters must be [A-Za-z0-9_] but found %q", v)
		}
	}

	// Evaluate the value
	if len(value) == 0 {
		// Empty values are ok! Shortcircuit
		return key, value, nil
	}

	// Scratch buffer for unescaped value
	newv := make([]byte, len(value))
	newi := 0
	// Track last significant character for trimming unquoted whitespace preceeding a trailing comment
	lastSig := 0

	// Parser State
	mode := normalMode

	for i := 0; i < len(value); i++ {
		v := value[i]

		// Control characters are always an error
		if v < 32 {
			return nil, nil, fmt.Errorf("0x%0.2x is an invalid value character", v)
		}

		// High bit set means it is part of a multibyte character, pass
		// it through as only ASCII characters have special meaning.
		if v > 127 {
			if mode == escapeMode {
				return nil, nil, ErrMultibyteEscape
			}
			// All multibyte characters are significant
			lastSig = newi
			newv[newi] = v
			newi++
			continue
		}

		switch mode {
		case normalMode:
			switch v {
			case '"':
				mode = doubleQuote
			case '\'':
				mode = singleQuote
			case '#':
				// Start of a comment, nothing left to parse
				return key, newv[:lastSig], nil
			case ' ', '\t':
				// Make sure whitespace doesn't get tracked
				newv[newi] = v
				newi++
			default:
				// Add the character to the new value
				newv[newi] = v
				newi++

				// Track last non-WS char for trimming on trailing comments
				lastSig = newi
			}
		case doubleQuote:
			switch v {
			case '"':
				mode = normalMode
			case '\\':
				mode = escapeMode
			default:
				// Add the character to the new value
				newv[newi] = v
				newi++

				// All quoted characters are significant
				lastSig = newi
			}
		case escapeMode:
			// We're in double quotes and the last character was a backslash
			switch v {
			case '"':
				newv[newi] = v
			case '\\':
				newv[newi] = v
			case '/':
				newv[newi] = v
			case 'b':
				newv[newi] = '\b'
			case 'f':
				newv[newi] = '\f'
			case 'r':
				newv[newi] = '\r'
			case 'n':
				newv[newi] = '\n'
			case 't':
				newv[newi] = '\t'
			case 'u':
				// Parse-ahead to capture unicode
				r, err := h2r(value[i+1:])
				if err != nil {
					return nil, nil, err
				}

				// Bump index by width of hex chars
				i += 4

				// Check if we need to get another rune
				if utf16.IsSurrogate(r) {
					if len(value) < i+6 {
						//TODO Use replacement character instead?
						return nil, nil, ErrIncompleteSur
					}
					if value[i+1] != '\\' || value[i+2] != 'u' {
						//TODO Use replacement character instead?
						return nil, nil, ErrIncompleteSur
					}

					r2, err := h2r(value[i+3:])
					if err != nil {
						return nil, nil, err
					}

					// Bump index by width of \uXXXX
					i += 6

					r = utf16.DecodeRune(r, r2)
				}
				n := utf8.EncodeRune(newv[newi:], r)
				newi += n - 1 // because it's incremented outside the switch
			default:
				return nil, nil, fmt.Errorf("invalid escape sequence: %q", string(v))
			}
			// Add the character to the new value
			newi++

			// All escaped characters are significant
			lastSig = newi

			// Switch back to quote mode
			mode = doubleQuote
		case singleQuote:
			switch v {
			case '\'':
				mode = normalMode
			default:
				// Add all other characters to the new value
				newv[newi] = v
				newi++

				// All single quoted characters are significant
				lastSig = newi
			}
		default:
			panic(fmt.Errorf("BUG: invalid mode: %v", mode))
		}
	}

	switch mode {
	case normalMode:
		// All escape sequences are complete and all quotes are matched
		return key, newv[:newi], nil
	case doubleQuote:
		return nil, nil, ErrUnmatchedDouble
	case singleQuote:
		return nil, nil, ErrUnmatchedSingle
	case escapeMode:
		return nil, nil, ErrIncompleteEscape
	default:
		panic(fmt.Errorf("BUG: invalid mode: %v", mode))
	}
}

// convert hex characters into a rune
func h2r(buf []byte) (rune, error) {
	if len(buf) < 4 {
		return 0, ErrIncompleteHex
	}
	var r rune
	for i := 0; i < 4; i++ {
		d := buf[i]
		switch {
		case '0' <= d && d <= '9':
			d = d - '0'
		case 'a' <= d && d <= 'f':
			d = d - 'a' + 10
		case 'A' <= d && d <= 'F':
			d = d - 'A' + 10
		default:
			return 0, fmt.Errorf("invalid hex character: %q", string(d))
		}

		r *= 16
		r += rune(d)
	}
	return r, nil
}

// Copyright (c) 2014, Kevin Walsh.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This code borrows heavily from the lexer design and implementation described
// by Rob Pike, "Lexical Scanning in Go", GTUG Sydney, Aug 30, 2011.
// See: http://cuddle.googlecode.com/hg/talk/lex.html#slide-40

package dlengine

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// token represents a value returned from the lexer.
type token struct {
	typ itemType // Type, such as itemNumber.
	val string   // Value, such as "23.2".
}

// itemType identifies the type of lex items.
type itemType int

// Comments: '%' to end of line (but not in strings), ignored
// Whitespace: ignored, except in strings
// Punctuation: '(’, ',’, ')’, ':-’, '.’, '~’, '?’, and '"’
// Note: We don't treat '=' specially or as punctuation, and we don't handle
// infix operators.

const (
	itemError itemType = iota // error occurred; value is text of error
	itemEOF
	itemQuestion // "?"
	itemWhen     // ":-"
	itemLP       // "("
	itemRP       // ")"
	itemComma    // ","
	// itemEqual   // "="  // TODO(kwalsh) support infix equality?
	itemDot        // "."
	itemTilde      // "~"
	itemVariable   // X, Alice, Hunter_22
	itemIdentifier // alice, 7, -42, x
	itemString     // "Alice"
)

func (i token) String() string {
	switch i.typ {
	case itemEOF:
		return "EOF"
	case itemError:
		return i.val
	case itemVariable:
		return fmt.Sprintf("var[%q]", i.val)
	case itemIdentifier:
		return fmt.Sprintf("ident[%q]", i.val)
	case itemString:
		return fmt.Sprintf("str[%q]", i.val)
	default:
		return fmt.Sprintf("punct[%q]", i.val)
	}
}

// stateFn represents a state transition for the scanner.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	name  string     // used only for error reports.
	input string     // the string being scanned.
	start int        // start position of this token.
	pos   int        // current position in the input.
	width int        // width of last rune read from input.
	state stateFn    // current state.
	items chan token // channel of scanned items.
	last  token      // last token returned by nextToken().
}

// emit passes an token back to the client.
func (l *lexer) emit(t itemType) {
	l.items <- token{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

const eof rune = 0

func lexMain(l *lexer) stateFn {
	// at the top level
	for {
		switch r := l.next(); {
		case r == eof:
			l.emit(itemEOF)
			return nil
		case unicode.IsSpace(r):
			l.ignore()
		case r == '%':
			l.backup()
			return lexComment
		case r == '.':
			l.emit(itemDot)
			return lexMain
		case r == '~':
			l.emit(itemTilde)
			return lexMain
		case r == '(':
			l.emit(itemLP)
			return lexMain
		case r == ')':
			l.emit(itemRP)
			return lexMain
		case r == '?':
			l.emit(itemQuestion)
			return lexMain
		case r == ':':
			l.backup()
			if !strings.HasPrefix(l.input[l.pos:], ":-") {
				return l.errorf(`expecting ":-"`)
			}
			l.pos += 2
			l.emit(itemWhen)
			return lexMain
		case r == ',':
			l.emit(itemComma)
			return lexMain
		case r == '"':
			l.backup()
			return lexString
		case 'A' <= r && r <= 'Z':
			l.backup()
			return lexVariable
		case unicode.IsPrint(r):
			l.backup()
			return lexIdentifier
		default:
			return l.errorf("unexpected rune: %v", r)
		}
	}
}

func lexString(l *lexer) stateFn {
	r := l.next()
	if r != '"' {
		return l.errorf("expecting '\"', got: %v", r)
	}
	escape := false
	for {
		switch r = l.next(); {
		case r == eof:
			return l.errorf("unexpected eof in string")
		case escape:
			escape = false
		case r == '\\':
			escape = true
		case r == '"':
			l.emit(itemString)
			return lexMain
		default:
			// continue loop
		}
	}
}

func variableRune(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || r == '_'
}

func lexVariable(l *lexer) stateFn {
	// precondition: l.next() is [A-Z]
	for {
		r := l.next()
		if r == eof || !variableRune(r) {
			l.backup()
			l.emit(itemVariable)
			return lexMain
		}
	}
}

func lexIdentifier(l *lexer) stateFn {
	// precondition: l.next() is printable, not banned punctuation, not [A-Z]
	invalid := `?:()~".,%` // '='
	for {
		r := l.next()
		if r == eof || unicode.IsSpace(r) || strings.IndexRune(invalid, r) >= 0 || !unicode.IsPrint(r) {
			l.backup()
			l.emit(itemIdentifier)
			return lexMain
		}
	}
}

func lexComment(l *lexer) stateFn {
	// precondition: l.next() is '%'
	for {
		r := l.next()
		if r == eof {
			l.backup()
			l.ignore()
			return lexMain
		} else if r == '\n' {
			l.ignore()
			return lexMain
		}
	}
}

// next returns the next rune in the input.
func (l *lexer) next() (r rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// backup steps back one rune. Can be called only once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
	l.width = 0
}

// errorf emits an error token and terminates the scan
// by returning nil as the next state.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- token{
		itemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	return &lexer{
		name:  name,
		input: input,
		state: lexMain,
		items: make(chan token, 2), // Two items is sufficient.
		last: token{itemEOF, ""},
	}
}

// nextToken returns the next token from the input.
func (l *lexer) nextToken() token {
	for {
		select {
		case l.last = <-l.items:
			return l.last
		default:
			if l.state == nil {
				// only happens for nextToken after itemEOF or itemError
				return l.last
			}
			l.state = l.state(l)
		}
	}
}

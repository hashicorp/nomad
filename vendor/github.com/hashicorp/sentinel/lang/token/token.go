// Portions of this file are taken directly from the Go source code
// licensed under the BSD 3-clause license. These lines are copyright
// 2009 The Go Authors.

// Package token defines constants representing the lexical tokens of the
// Sentinel policy language and basic operations on tokens.
//
package token

import "strconv"

// Token is the set of lexical tokens
type Token int

// The list of tokens.
const (
	// Special tokens
	ILLEGAL Token = iota
	EOF
	COMMENT

	literal_beg
	// Identifiers and basic type literals
	// (these tokens stand for classes of literals)
	IDENT  // main
	INT    // 12345
	FLOAT  // 123.45
	STRING // "abc"
	literal_end

	operator_beg
	// Operators and delimiters
	ADD // +
	SUB // -
	MUL // *
	QUO // /
	REM // %

	ADD_ASSIGN // +=
	SUB_ASSIGN // -=
	MUL_ASSIGN // *=
	QUO_ASSIGN // /=
	REM_ASSIGN // %=

	EQL // ==
	LSS // <
	GTR // >
	NEQ // !=
	LEQ // <=
	GEQ // >=

	ASSIGN // =

	NOT // !

	LPAREN // (
	LBRACK // [
	LBRACE // {

	RPAREN    // )
	RBRACK    // ]
	RBRACE    // }
	COMMA     // ,
	PERIOD    // .
	SEMICOLON // ;
	COLON     // :

	keyword_beg
	// Start keywords. These overlap with operators when they're directly text

	LAND   // and
	LOR    // or
	LXOR   // xor
	NOTSTR // not

	CONTAINS // contains
	IN       // in
	MATCHES  // matches
	IS       // is
	ISNOT    // is not

	ELSE // else is both a keyword and operator

	operator_end

	IMPORT // import
	AS     // as

	FUNC   // func
	RULE   // rule
	RETURN // return

	IF  // if
	ANY // any
	ALL // all
	FOR // for

	keyword_end
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",

	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:  "IDENT",
	INT:    "INT",
	FLOAT:  "FLOAT",
	STRING: "STRING",

	ADD: "+",
	SUB: "-",
	MUL: "*",
	QUO: "/",
	REM: "%",

	ADD_ASSIGN: "+=",
	SUB_ASSIGN: "-=",
	MUL_ASSIGN: "*=",
	QUO_ASSIGN: "/=",
	REM_ASSIGN: "%=",

	EQL: "==",
	LSS: "<",
	GTR: ">",
	NEQ: "!=",
	LEQ: "<=",
	GEQ: ">=",

	ASSIGN: "=",

	NOT:    "!",
	NOTSTR: "not",

	LPAREN: "(",
	LBRACK: "[",
	LBRACE: "{",

	RPAREN:    ")",
	RBRACK:    "]",
	RBRACE:    "}",
	COMMA:     ",",
	PERIOD:    ".",
	SEMICOLON: ";",
	COLON:     ":",

	LAND: "and",
	LOR:  "or",
	LXOR: "xor",

	CONTAINS: "contains",
	IN:       "in",
	MATCHES:  "matches",
	IS:       "is",
	ISNOT:    "is not",

	IMPORT: "import",
	AS:     "as",

	FUNC:   "func",
	RULE:   "rule",
	RETURN: "return",

	IF:   "if",
	ELSE: "else",
	ANY:  "any",
	ALL:  "all",
	FOR:  "for",
}

// String returns the string corresponding to the token tok.
// For operators, delimiters, and keywords the string is the actual
// token character sequence (e.g., for the token ADD, the string is
// "+"). For all other tokens the string corresponds to the token
// constant name (e.g. for the token IDENT, the string is "IDENT").
//
func (tok Token) String() string {
	s := ""
	if 0 <= tok && tok < Token(len(tokens)) {
		s = tokens[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

// A set of constants for precedence-based expression parsing.
// Non-operators have lowest precedence, followed by operators
// starting with precedence 1 up to unary operators. The highest
// precedence serves as "catch-all" precedence for selector,
// indexing, and other operator and delimiter tokens.
//
const (
	LowestPrec  = 0 // non-operators
	UnaryPrec   = 6
	HighestPrec = 7
)

// Precedence returns the operator precedence of the binary
// operator op. If op is not a binary operator, the result
// is LowestPrecedence.
//
func (op Token) Precedence() int {
	switch op {
	case LOR, LXOR:
		return 1
	case LAND:
		return 2
	case EQL, NEQ, LSS, LEQ, GTR, GEQ, IS, ISNOT, MATCHES, CONTAINS, IN:
		return 3
	case ELSE:
		return 4
	case ADD, SUB:
		return 5
	case MUL, QUO, REM:
		return 6
	}
	return LowestPrec
}

var keywords map[string]Token

func init() {
	keywords = make(map[string]Token)
	for i := keyword_beg + 1; i < keyword_end; i++ {
		keywords[tokens[i]] = i
	}
}

// Lookup maps an identifier to its keyword token or IDENT (if not a keyword).
//
func Lookup(ident string) Token {
	if tok, is_keyword := keywords[ident]; is_keyword {
		return tok
	}
	return IDENT
}

// Predicates

// IsLiteral returns true for tokens corresponding to identifiers
// and basic type literals; it returns false otherwise.
//
func (tok Token) IsLiteral() bool { return literal_beg < tok && tok < literal_end }

// IsOperator returns true for tokens corresponding to operators and
// delimiters; it returns false otherwise.
//
func (tok Token) IsOperator() bool { return operator_beg < tok && tok < operator_end }

// IsKeyword returns true for tokens corresponding to keywords;
// it returns false otherwise.
//
func (tok Token) IsKeyword() bool { return keyword_beg < tok && tok < keyword_end }

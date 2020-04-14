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

// This code borrows heavily from the lexer design and implementation for the
// template package. See http://golang.org/src/pkg/text/template/parse/parse.go

package dlengine

import (
	"fmt"
	"strconv"
	"strings"
)

// A node is an element in the parse tree.
type node interface {
	Type() nodeType
	String() string
	Copy() node
	Position() pos
}

// pos represents a byte position in the original input text from which
// this template was parsed.
type pos int

func (p pos) Position() pos {
	return p
}

// nodeType identifies the type of a parse tree node.
type nodeType int

func (t nodeType) Type() nodeType {
	return t
}

const (
	nodeProgram nodeType = iota // program ::= (assertion | retraction | query)*
	nodeAction                  // action ::= clause [ "." | "~" ]
	nodeQuery                   // query ::= literal "?"
	nodeClause                  // clause ::= literal | literal ":-" literal ("," literal)*
	nodeLiteral                 // literal ::= predsym | predsym "(" term ("," term)* ")"
	// These next few are left blank since they are not present in the parse tree:
	_              // nodePredSym ::= identifier | string
	_              // nodeTerm ::= variable | constant
	_              // nodeConstant ::= identifier | string
	nodeIdentifier // see lexer for syntax
	nodeString     // see lexer for syntax
	nodeVariable   // see lexer for syntax
)

// nodeList stores a list of nodes in the order they were lexed.
type nodeList []node

func (l *nodeList) append(n node) {
	*l = append(*l, n)
}

func (l *nodeList) join(sep string) string {
	if *l == nil {
		return ""
	}
	s := make([]string, len(*l))
	for i, n := range *l {
		s[i] = n.String()
	}
	return strings.Join(s, sep)
}

func (l *nodeList) dup() nodeList {
	if *l == nil {
		return *l
	}
	s := make([]node, len(*l))
	for i, n := range *l {
		s[i] = n.Copy()
	}
	return s
}

// programNode holds a sequence of assertion, retraction, and query nodes.
type programNode struct {
	nodeType
	pos
	nodeList
}

func newProgram(pos pos) *programNode {
	return &programNode{nodeProgram, pos, nil}
}

func (n *programNode) String() string {
	return n.join("\n")
}

func (n *programNode) Copy() node {
	return &programNode{nodeProgram, n.pos, n.nodeList.dup()}
}

// actionNode holds a clause and an actionType.
type actionNode struct {
	nodeType
	pos
	clause *clauseNode
	action actionType
}

type actionType bool

const actionAssert actionType = true
const actionRetract actionType = false

func newAction(pos pos, clause *clauseNode, action actionType) *actionNode {
	return &actionNode{nodeAction, pos, clause, action}
}

func (n *actionNode) String() string {
	var suffix string
	if n.action == actionAssert {
		suffix = "."
	} else {
		suffix = "~"
	}
	return n.clause.String() + suffix
}

func (n *actionNode) Copy() node {
	return &actionNode{nodeAction, n.pos, n.clause.Copy().(*clauseNode), n.action}
}

// queryNode holds a literal.
type queryNode struct {
	nodeType
	pos
	literal *literalNode
}

func newQuery(pos pos, literal *literalNode) *queryNode {
	return &queryNode{nodeQuery, pos, literal}
}

func (n *queryNode) String() string {
	return n.literal.String() + "?"
}

func (n *queryNode) Copy() node {
	return &queryNode{nodeQuery, n.pos, n.literal.Copy().(*literalNode)}
}

// clauseNode holds a head literal and a sequence of body literals.
type clauseNode struct {
	nodeType
	pos
	head *literalNode
	nodeList
}

func newClause(pos pos, head *literalNode) *clauseNode {
	return &clauseNode{nodeClause, pos, head, nil}
}

func (n *clauseNode) String() string {
	if len(n.nodeList) == 0 {
		return n.head.String()
	}
	return n.head.String() + " :- " + n.join(", ")
}

func (n *clauseNode) Copy() node {
	return &clauseNode{nodeClause, n.pos, n.head.Copy().(*literalNode), n.nodeList.dup()}
}

// literalNode holds a predsym and a sequence of terms.
type literalNode struct {
	nodeType
	pos
	predsym string
	nodeList
}

func newLiteral(pos pos, predsym string) *literalNode {
	return &literalNode{nodeLiteral, pos, predsym, nil}
}

func (n *literalNode) String() string {
	if len(n.nodeList) == 0 {
		return n.predsym
	}
	return n.predsym + "(" + n.join(", ") + ")"
}

func (n *literalNode) Copy() node {
	return &literalNode{nodeLiteral, n.pos, n.predsym, n.nodeList.dup()}
}

// leafNode holds a string.
type leafNode struct {
	nodeType
	pos
	val string
}

func newLeaf(t nodeType, pos pos, val string) *leafNode {
	return &leafNode{t, pos, val}
}

func (n *leafNode) String() string {
	if n.nodeType == nodeString {
		return strconv.Quote(n.val)
	} else {
		return n.val
	}
}

func (n *leafNode) Copy() node {
	return &leafNode{n.nodeType, n.pos, n.val}
}

// parser holds the state of the recursive descent parser.
type parser struct {
	lex   *lexer
	pos   pos
	token token // single-token lookahead.
}

func (parser *parser) next() {
	parser.pos = pos(parser.lex.pos)
	parser.token = parser.lex.nextToken()
}

func (parser *parser) parseTerm() (node, error) {
	var n node
	switch parser.token.typ {
	case itemVariable:
		n = newLeaf(nodeVariable, parser.pos, parser.token.val)
	case itemIdentifier:
		n = newLeaf(nodeIdentifier, parser.pos, parser.token.val)
	case itemString:
		s, err := strconv.Unquote(parser.token.val)
		if err != nil {
			return nil, fmt.Errorf("datalog: improperly quoted string: %v", parser.token.val)
		}
		n = newLeaf(nodeString, parser.pos, s)
	default:
		return nil, fmt.Errorf("datalog: expecting variable or constant, found: %v", parser.token)
	}
	parser.next()
	return n, nil
}

func (parser *parser) parseLiteral() (*literalNode, error) {
	if parser.token.typ != itemIdentifier && parser.token.typ != itemString {
		return nil, fmt.Errorf("datalog: expecting identifier or string, found: %v", parser.token)
	}
	literal := newLiteral(parser.pos, parser.token.val)
	parser.next()
	if parser.token.typ != itemLP {
		return literal, nil
	}
	parser.next()
	term, err := parser.parseTerm()
	if err != nil {
		return nil, err
	}
	literal.append(term)
	for parser.token.typ != itemRP {
		if parser.token.typ != itemComma {
			return nil, fmt.Errorf("datalog: expecting ',' or ')', found: %v", parser.token)
		}
		parser.next()
		term, err = parser.parseTerm()
		if err != nil {
			return nil, err
		}
		literal.append(term)
	}
	parser.next()
	return literal, nil
}

func parse(name, input string) (*programNode, error) {
	l := lex(name, input)
	parser := &parser{lex: l}
	parser.next()
	pgm := newProgram(parser.pos)
	for {
		switch parser.token.typ {
		case itemEOF:
			return pgm, nil
		default:
			literal, err := parser.parseLiteral()
			if err != nil {
				return nil, err
			}
			if parser.token.typ == itemQuestion {
				pgm.append(newQuery(parser.pos, literal))
				parser.next()
			} else {
				clause := newClause(parser.pos, literal)
				if parser.token.typ == itemWhen {
					parser.next()
					body, err := parser.parseLiteral()
					if err != nil {
						return nil, err
					}
					clause.append(body)
					for parser.token.typ == itemComma {
						parser.next()
						body, err = parser.parseLiteral()
						if err != nil {
							return nil, err
						}
						clause.append(body)
					}
				}
				if parser.token.typ == itemDot {
					pgm.append(newAction(parser.pos, clause, actionAssert))
					parser.next()
				} else if parser.token.typ == itemTilde {
					pgm.append(newAction(parser.pos, clause, actionRetract))
					parser.next()
				} else {
					return nil, fmt.Errorf("datalog: unexpected: %v", parser.token)
				}
			}
		}
	}
}

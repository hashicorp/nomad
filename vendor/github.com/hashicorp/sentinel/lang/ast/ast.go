// Portions of this file are taken directly from the Go source code
// licensed under the BSD 3-clause license. These lines are copyright
// 2009 The Go Authors.

// Package ast declares the types used to represent syntax trees for Sentinel.
package ast

import (
	"strings"

	"github.com/hashicorp/sentinel/lang/token"
)

// ----------------------------------------------------------------------------
// Interfaces
//
// There are 3 main classes of nodes: Expressions nodes,
// statement nodes, and declaration nodes. The node names usually
// match the corresponding language spec production names to which they
// correspond. The node fields correspond to the individual parts
// of the respective productions.
//
// All nodes contain position information marking the beginning of
// the corresponding source text segment; it is accessible via the
// Pos accessor method. Nodes may contain additional position info
// for language constructs where comments may be found between parts
// of the construct (typically any larger, parenthesized subpart).
// That position information is needed to properly position comments
// when printing the construct.

// All node types implement the Node interface.
type Node interface {
	Pos() token.Pos // position of first character belonging to the node
	End() token.Pos // position of first character immediately after the node
}

// All expression nodes implement the Expr interface.
type Expr interface {
	Node

	ExprNode()
}

// All statement nodes implement the Stmt interface.
type Stmt interface {
	Node

	StmtNode()
}

// ----------------------------------------------------------------------------
// Comments

// A Comment node represents a single #-style, //-style, or /*-style comment.
type Comment struct {
	Slash token.Pos // position of character starting the comment
	Text  string    // comment text (excluding '\n' for #-style or //-style comments)
}

func (c *Comment) Pos() token.Pos { return c.Slash }
func (c *Comment) End() token.Pos { return token.Pos(int(c.Slash) + len(c.Text)) }

// A CommentGroup represents a sequence of comments with no other tokens and
// no empty lines between.
type CommentGroup struct {
	List []*Comment // len(List) > 0
}

func (g *CommentGroup) Pos() token.Pos { return g.List[0].Pos() }
func (g *CommentGroup) End() token.Pos { return g.List[len(g.List)-1].End() }

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[0:i]
}

// Text returns the text of the comment.
// Comment markers (//, /*, and */), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty lines are
// reduced to one, and trailing space on lines is trimmed.
//
func (g *CommentGroup) Text() string {
	if g == nil {
		return ""
	}
	comments := make([]string, len(g.List))
	for i, c := range g.List {
		comments[i] = string(c.Text)
	}

	lines := make([]string, 0, 10) // most comments are less than 10 lines
	for _, c := range comments {
		// Remove comment markers.
		// The parser has given us exactly the comment text.
		if c[0] == '#' {
			// #-style comment
			c = c[1:]

			// strip first space
			if len(c) > 0 && c[0] == ' ' {
				c = c[1:]
			}
		} else {
			switch c[1] {
			case '/':
				//-style comment (no newline at the end)
				c = c[2:]
				// strip first space - required for Example tests
				if len(c) > 0 && c[0] == ' ' {
					c = c[1:]
				}
			case '*':
				/*-style comment */
				c = c[2 : len(c)-2]
			}
		}

		// Split on newlines.
		cl := strings.Split(c, "\n")

		// Walk lines, stripping trailing white space and adding to list.
		for _, l := range cl {
			lines = append(lines, stripTrailingWhitespace(l))
		}
	}

	// Remove leading blank lines; convert runs of
	// interior blank lines to a single blank line.
	n := 0
	for _, line := range lines {
		if line != "" || n > 0 && lines[n-1] != "" {
			lines[n] = line
			n++
		}
	}
	lines = lines[0:n]

	return strings.Join(lines, "\n")
}

// ----------------------------------------------------------------------------
// Expressions and types

// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.
type (
	// A BadExpr node is a placeholder for expressions containing
	// syntax errors for which no correct expression nodes can be
	// created.
	//
	BadExpr struct {
		From, To token.Pos // position range of bad expression
	}

	// An Ident node represents an identifier.
	Ident struct {
		NamePos token.Pos // identifier position
		Name    string    // identifier name
		Obj     *Object   // denoted object; or nil
	}

	// A BasicLit node represents a literal of basic type.
	BasicLit struct {
		ValuePos token.Pos   // literal position
		Kind     token.Token // token.INT, token.FLOAT, or token.STRING
		Value    string      // literal string; e.g. 42, 0x7f, 3.14, 1e-9, "foo"
	}

	// FuncLit represents a function literal.
	FuncLit struct {
		Func    token.Pos  // position of "func" keyword
		Opening token.Pos  // position of opening parenthesis
		Params  []*Ident   // List of param identifiers
		Closing token.Pos  // position of closing parenthesis
		Body    *BlockStmt // rule body
	}

	// A ListLit node represents a literal list.
	ListLit struct {
		Lbrack token.Pos // position of "["
		Elts   []Expr    // list of elements; or nil
		Rbrack token.Pos // position of "]"
	}

	// A MapLit node represents a literal map.
	MapLit struct {
		Lbrace token.Pos // position of "{"
		Elts   []Expr    // list of elements; or nil
		Rbrace token.Pos // position of "}"
	}

	// A QuantExpr represents a quantifier expression.
	QuantExpr struct {
		Op     token.Token // operator
		OpPos  token.Pos   // position of Op
		Expr   Expr        // expression
		Name   *Ident      // ident to assign
		Name2  *Ident      // second ident or nil
		Lbrace token.Pos   // position of "{"
		Body   Expr
		Rbrace token.Pos // position of "}"
	}

	// A UnaryExpr node represents a unary expression.
	//
	UnaryExpr struct {
		OpPos token.Pos   // position of Op
		Op    token.Token // operator
		X     Expr        // operand
	}

	// A BinaryExpr node represents a binary expression.
	BinaryExpr struct {
		X     Expr        // left operand
		OpPos token.Pos   // position of Op
		Op    token.Token // operator
		OpNeg bool        // negated operator ("not in")
		Y     Expr        // right operand
	}

	// A ParenExpr node represents a parenthesized expression.
	ParenExpr struct {
		Lparen token.Pos // position of "("
		X      Expr      // parenthesized expression
		Rparen token.Pos // position of ")"
	}

	// A SelectorExpr node represents an expression followed by a selector.
	SelectorExpr struct {
		X   Expr   // expression
		Sel *Ident // field selector
	}

	// An IndexExpr node represents an expression followed by an index.
	IndexExpr struct {
		X      Expr      // expression
		Lbrack token.Pos // position of "["
		Index  Expr      // index expression
		Rbrack token.Pos // position of "]"
	}

	// An SliceExpr node represents an expression followed by slice indices.
	SliceExpr struct {
		X      Expr      // expression
		Lbrack token.Pos // position of "["
		Low    Expr      // begin of slice range; or nil
		High   Expr      // end of slice range; or nil
		Rbrack token.Pos // position of "]"
	}

	// A RuleLit represents a rule.
	RuleLit struct {
		Rule   token.Pos // position of "rule" keyword
		When   Expr      // when predicate if not nil
		Lbrace token.Pos // position of "{"
		Expr   Expr
		Rbrace token.Pos // position of "}"
	}

	// A CallExpr node represents an expression followed by an argument list.
	CallExpr struct {
		Fun    Expr      // function expression
		Lparen token.Pos // position of "("
		Args   []Expr    // function arguments; or nil
		Rparen token.Pos // position of ")"
	}

	// A KeyValueExpr node represents (key : value) pairs
	// in composite literals.
	//
	KeyValueExpr struct {
		Key   Expr
		Colon token.Pos // position of ":"
		Value Expr
	}
)

// Pos and End implementations for expression/type nodes.
//
func (x *BadExpr) Pos() token.Pos      { return x.From }
func (x *Ident) Pos() token.Pos        { return x.NamePos }
func (x *BasicLit) Pos() token.Pos     { return x.ValuePos }
func (x *FuncLit) Pos() token.Pos      { return x.Func }
func (x *ListLit) Pos() token.Pos      { return x.Lbrack }
func (x *MapLit) Pos() token.Pos       { return x.Lbrace }
func (x *QuantExpr) Pos() token.Pos    { return x.OpPos }
func (x *UnaryExpr) Pos() token.Pos    { return x.OpPos }
func (x *BinaryExpr) Pos() token.Pos   { return x.X.Pos() }
func (x *ParenExpr) Pos() token.Pos    { return x.Lparen }
func (x *SelectorExpr) Pos() token.Pos { return x.X.Pos() }
func (x *IndexExpr) Pos() token.Pos    { return x.X.Pos() }
func (x *SliceExpr) Pos() token.Pos    { return x.X.Pos() }
func (x *RuleLit) Pos() token.Pos      { return x.Rule }
func (x *CallExpr) Pos() token.Pos     { return x.Fun.Pos() }
func (x *KeyValueExpr) Pos() token.Pos { return x.Key.Pos() }

func (x *BadExpr) End() token.Pos      { return x.To }
func (x *Ident) End() token.Pos        { return token.Pos(int(x.NamePos) + len(x.Name)) }
func (x *BasicLit) End() token.Pos     { return token.Pos(int(x.ValuePos) + len(x.Value)) }
func (x *FuncLit) End() token.Pos      { return x.Body.End() }
func (x *ListLit) End() token.Pos      { return x.Rbrack + 1 }
func (x *MapLit) End() token.Pos       { return x.Rbrace + 1 }
func (x *QuantExpr) End() token.Pos    { return x.Rbrace + 1 }
func (x *UnaryExpr) End() token.Pos    { return x.X.End() }
func (x *BinaryExpr) End() token.Pos   { return x.Y.End() }
func (x *ParenExpr) End() token.Pos    { return x.Rparen + 1 }
func (x *SelectorExpr) End() token.Pos { return x.Sel.End() }
func (x *IndexExpr) End() token.Pos    { return x.Rbrack + 1 }
func (x *SliceExpr) End() token.Pos    { return x.Rbrack + 1 }
func (x *RuleLit) End() token.Pos      { return x.Rbrace }
func (x *CallExpr) End() token.Pos     { return x.Rparen + 1 }
func (x *KeyValueExpr) End() token.Pos { return x.Value.End() }

// exprNode() ensures that only expression/type nodes can be
// assigned to an Expr.
//
func (*BadExpr) ExprNode()      {}
func (*Ident) ExprNode()        {}
func (*BasicLit) ExprNode()     {}
func (*FuncLit) ExprNode()      {}
func (*ListLit) ExprNode()      {}
func (*MapLit) ExprNode()       {}
func (*QuantExpr) ExprNode()    {}
func (*UnaryExpr) ExprNode()    {}
func (*BinaryExpr) ExprNode()   {}
func (*ParenExpr) ExprNode()    {}
func (*SelectorExpr) ExprNode() {}
func (*IndexExpr) ExprNode()    {}
func (*SliceExpr) ExprNode()    {}
func (*RuleLit) ExprNode()      {}
func (*CallExpr) ExprNode()     {}
func (*KeyValueExpr) ExprNode() {}

// ----------------------------------------------------------------------------
// Convenience functions for Idents

// NewIdent creates a new Ident without position.
// Useful for ASTs generated by code other than the Go parser.
//
func NewIdent(name string) *Ident { return &Ident{token.NoPos, name, nil} }

func (id *Ident) String() string {
	if id != nil {
		return id.Name
	}
	return "<nil>"
}

// ----------------------------------------------------------------------------
// Statements

// A statement is represented by a tree consisting of one
// or more of the following concrete statement nodes.
//
type (
	// A BadStmt node is a placeholder for statements containing
	// syntax errors for which no correct statement nodes can be
	// created.
	//
	BadStmt struct {
		From, To token.Pos // position range of bad statement
	}

	// An EmptyStmt node represents an empty statement.
	// The "position" of the empty statement is the position
	// of the immediately following (explicit or implicit) semicolon.
	//
	EmptyStmt struct {
		Semicolon token.Pos // position of following ";"
		Implicit  bool      // if set, ";" was omitted in the source
	}

	// AssignStmt node represents an assignment
	AssignStmt struct {
		Lhs    Expr
		TokPos token.Pos   // position of Tok
		Tok    token.Token // assignment token
		Rhs    Expr
	}

	// A BlockStmt node represents a braced statement list.
	BlockStmt struct {
		Lbrace token.Pos // position of "{"
		List   []Stmt
		Rbrace token.Pos // position of "}"
	}

	// An IfStmt node represents an if statement.
	IfStmt struct {
		If   token.Pos // position of "if" keyword
		Cond Expr      // condition
		Body *BlockStmt
		Else Stmt // else branch; or nil
	}

	// A ForStmt represents a for statement.
	ForStmt struct {
		For   token.Pos // position of "for" keyword
		Expr  Expr      // expression
		Name  *Ident    // ident to assign
		Name2 *Ident    // second ident or nil
		Body  *BlockStmt
	}

	// A ReturnStmt node represents a return statement.
	ReturnStmt struct {
		Return token.Pos // position of "return" keyword
		Result Expr      // result expression; never nil
	}

	// An ExprStmt node represents a (stand-alone) expression
	// in a statement list.
	//
	ExprStmt struct {
		X Expr // expression
	}
)

// Pos and End implementations for statement nodes.
//
func (s *BadStmt) Pos() token.Pos    { return s.From }
func (s *EmptyStmt) Pos() token.Pos  { return s.Semicolon }
func (s *AssignStmt) Pos() token.Pos { return s.Lhs.Pos() }
func (s *BlockStmt) Pos() token.Pos  { return s.Lbrace }
func (s *IfStmt) Pos() token.Pos     { return s.If }
func (s *ForStmt) Pos() token.Pos    { return s.For }
func (s *ReturnStmt) Pos() token.Pos { return s.Return }
func (s *ExprStmt) Pos() token.Pos   { return s.X.Pos() }

func (s *BadStmt) End() token.Pos { return s.To }
func (s *EmptyStmt) End() token.Pos {
	if s.Implicit {
		return s.Semicolon
	}
	return s.Semicolon + 1 /* len(";") */
}
func (s *AssignStmt) End() token.Pos { return s.Rhs.End() }
func (s *BlockStmt) End() token.Pos  { return s.Rbrace + 1 }
func (s *IfStmt) End() token.Pos {
	if s.Else != nil {
		return s.Else.End()
	}
	return s.Body.End()
}
func (s *ForStmt) End() token.Pos    { return s.Body.End() }
func (s *ReturnStmt) End() token.Pos { return s.Result.End() }
func (s *ExprStmt) End() token.Pos   { return s.X.End() }

// stmtNode() ensures that only statement nodes can be
// assigned to a Stmt.
//
func (*BadStmt) StmtNode()    {}
func (*EmptyStmt) StmtNode()  {}
func (*AssignStmt) StmtNode() {}
func (*BlockStmt) StmtNode()  {}
func (*IfStmt) StmtNode()     {}
func (*ForStmt) StmtNode()    {}
func (*ReturnStmt) StmtNode() {}
func (*ExprStmt) StmtNode()   {}

// ----------------------------------------------------------------------------
// Declarations

// A Spec node represents a single (non-parenthesized) import,
// constant, type, or variable declaration.
//
type (
	// The Spec type stands for any of *ImportSpec, *ValueSpec
	Spec interface {
		Node
		specNode()
	}

	// An ImportSpec node represents a single package import.
	ImportSpec struct {
		Doc     *CommentGroup // associated documentation; or nil
		Name    *Ident        // local package name (including "."); or nil
		Path    *BasicLit     // import path
		Comment *CommentGroup // line comments; or nil
		EndPos  token.Pos     // end of spec (overrides Path.Pos if nonzero)
	}
)

// Pos and End implementations for spec nodes.
//
func (s *ImportSpec) Pos() token.Pos {
	if s.Name != nil {
		return s.Name.Pos()
	}
	return s.Path.Pos()
}

func (s *ImportSpec) End() token.Pos {
	if s.EndPos != 0 {
		return s.EndPos
	}
	return s.Path.End()
}

// specNode() ensures that only spec nodes can be
// assigned to a Spec.
//
func (*ImportSpec) specNode() {}

// ----------------------------------------------------------------------------
// Files and packages

// A File node represents a Sentinel source file.
//
// The Comments list contains all comments in the source file in order of
// appearance, including the comments that are pointed to from other nodes
// via Doc and Comment fields.
type File struct {
	Doc        *CommentGroup   // Policy description
	Scope      *Scope          // package scope (this file only)
	Imports    []*ImportSpec   // imports in this file
	Stmts      []Stmt          // top-level statements, or nil if empty
	Unresolved []*Ident        // unresolved identifiers in this file
	Comments   []*CommentGroup // list of all comments in the source file
}

func (f *File) Pos() token.Pos {
	if len(f.Imports) > 0 {
		return f.Imports[0].Pos()
	}

	if len(f.Stmts) > 0 {
		return f.Stmts[0].Pos()
	}

	return token.NoPos
}

func (f *File) End() token.Pos {
	if n := len(f.Stmts); n > 0 {
		return f.Stmts[n-1].End()
	}

	return token.NoPos
}

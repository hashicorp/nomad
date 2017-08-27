// Portions of this file are taken directly from the Go source code
// licensed under the BSD 3-clause license. These lines are copyright
// 2009 The Go Authors.

// Package parser implements a parser for Sentinel source files. Input may be
// provided in a variety of forms (see the various Parse* functions); the
// output is an abstract syntax tree (AST) representing the source. The
// parser is invoked through one of the Parse* functions.
//
// The parser may accept a larger language than is syntactically permitted by
// the actual language, for simplicity, and for improved robustness in the presence
// of syntax errors. These errors will be caught during the semantic check
// phases of the langauge.
//
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/scanner"
	"github.com/hashicorp/sentinel/lang/token"
)

// The parser structure holds the parser's internal state.
type parser struct {
	file    *token.File
	errors  scanner.ErrorList
	scanner scanner.Scanner

	// Tracing/debugging
	mode   Mode // parsing mode
	trace  bool // == (mode & Trace != 0)
	indent int  // indentation used for tracing output

	// Comments
	comments    []*ast.CommentGroup
	leadComment *ast.CommentGroup // last lead comment
	lineComment *ast.CommentGroup // last line comment

	// Next token
	pos token.Pos   // token position
	tok token.Token // one token look-ahead
	lit string      // token literal

	// Error recovery
	// (used to limit the number of calls to syncXXX functions
	// w/o making scanning progress - avoids potential endless
	// loops across multiple parser functions during error recovery)
	syncPos token.Pos // last synchronization position
	syncCnt int       // number of calls to syncXXX without progress

	// Non-syntactic parser control
	exprLev int  // < 0: in control clause, >= 0: in expression
	inRhs   bool // if set, the parser is parsing a rhs expression

	// Ordinary identifier scopes
	pkgScope   *ast.Scope   // pkgScope.Outer == nil
	topScope   *ast.Scope   // top-most scope; may be pkgScope
	unresolved []*ast.Ident // unresolved identifiers
}

func (p *parser) init(fset *token.FileSet, filename string, src []byte, mode Mode) {
	p.file = fset.AddFile(filename, -1, len(src))

	eh := func(pos token.Position, msg string) { p.errors.Add(pos, msg) }
	p.scanner.Init(p.file, src, eh, scanner.ScanComments)

	p.mode = mode
	p.trace = mode&Trace != 0 // for convenience (p.trace is used frequently)

	p.next()
}

// ----------------------------------------------------------------------------
// Scoping support

func (p *parser) openScope() {
	p.topScope = ast.NewScope(p.topScope)
}

func (p *parser) closeScope() {
	p.topScope = p.topScope.Outer
}

func (p *parser) declare(decl interface{}, scope *ast.Scope, kind ast.ObjKind, idents ...*ast.Ident) {
	for _, ident := range idents {
		assert(ident.Obj == nil, "identifier already declared or resolved")
		obj := ast.NewObj(kind, ident.Name)

		// remember the corresponding declaration for redeclaration
		// errors and global variable resolution/typechecking phase
		obj.Decl = decl
		ident.Obj = obj
		if ident.Name != "_" {
			if alt := scope.Insert(obj); alt != nil && p.mode&DeclarationErrors != 0 {
				prevDecl := ""
				if pos := alt.Pos(); pos.IsValid() {
					prevDecl = fmt.Sprintf("\n\tprevious declaration at %s", p.file.Position(pos))
				}
				p.error(ident.Pos(), fmt.Sprintf("%s redeclared in this block%s", ident.Name, prevDecl))
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Parsing support

func (p *parser) printTrace(a ...interface{}) {
	const dots = ". . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . "
	const n = len(dots)
	pos := p.file.Position(p.pos)
	fmt.Printf("%5d:%3d: ", pos.Line, pos.Column)
	i := 2 * p.indent
	for i > n {
		fmt.Print(dots)
		i -= n
	}
	// i <= n
	fmt.Print(dots[0:i])
	fmt.Println(a...)
}

func trace(p *parser, msg string) *parser {
	p.printTrace(msg, "(")
	p.indent++
	return p
}

// Usage pattern: defer un(trace(p, "..."))
func un(p *parser) {
	p.indent--
	p.printTrace(")")
}

// Advance to the next token.
func (p *parser) next0() {
	// Because of one-token look-ahead, print the previous token
	// when tracing as it provides a more readable output. The
	// very first token (!p.pos.IsValid()) is not initialized
	// (it is token.ILLEGAL), so don't print it .
	if p.trace && p.pos.IsValid() {
		s := p.tok.String()
		switch {
		case p.tok.IsLiteral():
			p.printTrace(s, p.lit)
		case p.tok.IsOperator(), p.tok.IsKeyword():
			p.printTrace("\"" + s + "\"")
		default:
			p.printTrace(s)
		}
	}

	p.pos, p.tok, p.lit = p.scanner.Scan()
}

// Consume a comment and return it and the line on which it ends.
func (p *parser) consumeComment() (comment *ast.Comment, endline int) {
	// /*-style comments may end on a different line than where they start.
	// Scan the comment for '\n' chars and adjust endline accordingly.
	endline = p.file.Line(p.pos)
	if p.lit[1] == '*' {
		// don't use range here - no need to decode Unicode code points
		for i := 0; i < len(p.lit); i++ {
			if p.lit[i] == '\n' {
				endline++
			}
		}
	}

	comment = &ast.Comment{Slash: p.pos, Text: p.lit}
	p.next0()

	return
}

// Consume a group of adjacent comments, add it to the parser's
// comments list, and return it together with the line at which
// the last comment in the group ends. A non-comment token or n
// empty lines terminate a comment group.
//
func (p *parser) consumeCommentGroup(n int) (comments *ast.CommentGroup, endline int) {
	var list []*ast.Comment
	endline = p.file.Line(p.pos)
	for p.tok == token.COMMENT && p.file.Line(p.pos) <= endline+n {
		var comment *ast.Comment
		comment, endline = p.consumeComment()
		list = append(list, comment)
	}

	// add comment group to the comments list
	comments = &ast.CommentGroup{List: list}
	p.comments = append(p.comments, comments)

	return
}

// Advance to the next non-comment token. In the process, collect
// any comment groups encountered, and remember the last lead and
// and line comments.
//
// A lead comment is a comment group that starts and ends in a
// line without any other tokens and that is followed by a non-comment
// token on the line immediately after the comment group.
//
// A line comment is a comment group that follows a non-comment
// token on the same line, and that has no tokens after it on the line
// where it ends.
//
// Lead and line comments may be considered documentation that is
// stored in the AST.
//
func (p *parser) next() {
	p.leadComment = nil
	p.lineComment = nil
	prev := p.pos
	p.next0()

	if p.tok == token.COMMENT {
		var comment *ast.CommentGroup
		var endline int

		if p.file.Line(p.pos) == p.file.Line(prev) {
			// The comment is on same line as the previous token; it
			// cannot be a lead comment but may be a line comment.
			comment, endline = p.consumeCommentGroup(0)
			if p.file.Line(p.pos) != endline {
				// The next token is on a different line, thus
				// the last comment group is a line comment.
				p.lineComment = comment
			}
		}

		// consume successor comments, if any
		endline = -1
		for p.tok == token.COMMENT {
			comment, endline = p.consumeCommentGroup(1)
		}

		if endline+1 == p.file.Line(p.pos) {
			// The next token is following on the line immediately after the
			// comment group, thus the last comment group is a lead comment.
			p.leadComment = comment
		}
	}
}

// A bailout panic is raised to indicate early termination.
type bailout struct{}

func (p *parser) error(pos token.Pos, msg string) {
	epos := p.file.Position(pos)

	// If AllErrors is not set, discard errors reported on the same line
	// as the last recorded error and stop parsing if there are more than
	// 10 errors.
	if p.mode&AllErrors == 0 {
		n := len(p.errors)
		if n > 0 && p.errors[n-1].Pos.Line == epos.Line {
			return // discard - likely a spurious error
		}
		if n > 10 {
			panic(bailout{})
		}
	}

	p.errors.Add(epos, msg)
}

func (p *parser) errorExpected(pos token.Pos, msg string) {
	msg = "expected " + msg
	if pos == p.pos {
		// the error happened at the current position;
		// make the error message more specific
		if p.tok == token.SEMICOLON && p.lit == "\n" {
			msg += ", found newline"
		} else {
			msg += ", found '" + p.tok.String() + "'"
			if p.tok.IsLiteral() {
				msg += " " + p.lit
			}
		}
	}
	p.error(pos, msg)
}

func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.pos
	if p.tok != tok {
		p.errorExpected(pos, "'"+tok.String()+"'")
	}
	p.next() // make progress
	return pos
}

// expectClosing is like expect but provides a better error message
// for the common case of a missing comma before a newline.
//
func (p *parser) expectClosing(tok token.Token, context string) token.Pos {
	if p.tok != tok && p.tok == token.SEMICOLON && p.lit == "\n" {
		p.error(p.pos, "missing ',' before newline in "+context)
		p.next()
	}
	return p.expect(tok)
}

func (p *parser) expectSemi() {
	// semicolon is optional before a closing ')' or '}'
	if p.tok != token.RPAREN && p.tok != token.RBRACE {
		switch p.tok {
		case token.COMMA:
			// permit a ',' instead of a ';' but complain
			p.errorExpected(p.pos, "';'")
			fallthrough
		case token.SEMICOLON:
			p.next()
		default:
			p.errorExpected(p.pos, "';'")
			//TODO:syncStmt(p)
		}
	}
}

func (p *parser) atComma(context string, follow token.Token) bool {
	if p.tok == token.COMMA {
		return true
	}
	if p.tok != follow {
		msg := "missing ','"
		if p.tok == token.SEMICOLON && p.lit == "\n" {
			msg += " before newline"
		}
		p.error(p.pos, msg+" in "+context)
		return true // "insert" comma and continue
	}
	return false
}

func assert(cond bool, msg string) {
	if !cond {
		panic("go/parser internal error: " + msg)
	}
}

// safePos returns a valid file position for a given position: If pos
// is valid to begin with, safePos returns pos. If pos is out-of-range,
// safePos returns the EOF position.
//
// This is hack to work around "artificial" end positions in the AST which
// are computed by adding 1 to (presumably valid) token positions. If the
// token positions are invalid due to parse errors, the resulting end position
// may be past the file's EOF position, which would lead to panics if used
// later on.
//
func (p *parser) safePos(pos token.Pos) (res token.Pos) {
	defer func() {
		if recover() != nil {
			res = token.Pos(p.file.Base() + p.file.Size()) // EOF position
		}
	}()
	_ = p.file.Offset(pos) // trigger a panic if position is out-of-range
	return pos
}

// ----------------------------------------------------------------------------
// Error sync functions

// syncStmt advances to the next statement.
// Used for synchronization after an error.
//
func syncStmt(p *parser) {
	for {
		switch p.tok {
		case token.IF, token.ALL, token.ANY:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 sync calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop (it is possible that
			// both parseOperand and parseStmt call syncStmt and
			// correctly do not advance, thus the need for the
			// invocation limit p.syncCnt).
			if p.pos == p.syncPos && p.syncCnt < 10 {
				p.syncCnt++
				return
			}
			if p.pos > p.syncPos {
				p.syncPos = p.pos
				p.syncCnt = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}

		p.next()
	}
}

// syncDecl advances to the next declaration.
// Used for synchronization after an error.
//
func syncDecl(p *parser) {
	for {
		switch p.tok {
		case token.IDENT:
			// see comments in syncStmt
			if p.pos == p.syncPos && p.syncCnt < 10 {
				p.syncCnt++
				return
			}
			if p.pos > p.syncPos {
				p.syncPos = p.pos
				p.syncCnt = 0
				return
			}
		case token.EOF:
			return
		}
		p.next()
	}
}

// ----------------------------------------------------------------------------
// Resolution

// The unresolved object is a sentinel to mark identifiers that have been added
// to the list of unresolved identifiers. The sentinel is only used for verifying
// internal consistency.
var unresolved = new(ast.Object)

// If x is an identifier, tryResolve attempts to resolve x by looking up
// the object it denotes. If no object is found and collectUnresolved is
// set, x is marked as unresolved and collected in the list of unresolved
// identifiers.
//
func (p *parser) tryResolve(x ast.Expr, collectUnresolved bool) {
	// nothing to do if x is not an identifier or the blank identifier
	ident, _ := x.(*ast.Ident)
	if ident == nil {
		return
	}

	assert(ident.Obj == nil, "identifier already declared or resolved")
	if ident.Name == "_" {
		return
	}

	// try to resolve the identifier
	for s := p.topScope; s != nil; s = s.Outer {
		if obj := s.Lookup(ident.Name); obj != nil {
			ident.Obj = obj
			return
		}
	}

	// all local scopes are known, so any unresolved identifier
	// must be found either in the file scope, package scope
	// (perhaps in another file), or universe scope --- collect
	// them so that they can be resolved later
	if collectUnresolved {
		ident.Obj = unresolved
		p.unresolved = append(p.unresolved, ident)
	}
}

func (p *parser) resolve(x ast.Expr) {
	p.tryResolve(x, true)
}

// ----------------------------------------------------------------------------
// Identifiers

func (p *parser) parseIdent() *ast.Ident {
	pos := p.pos
	name := "_"
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else {
		p.expect(token.IDENT) // use expect() error handling
	}

	return &ast.Ident{NamePos: pos, Name: name}
}

// ----------------------------------------------------------------------------
// Blocks

func (p *parser) parseStmtList() (list []ast.Stmt) {
	if p.trace {
		defer un(trace(p, "StatementList"))
	}

	for p.tok != token.RBRACE && p.tok != token.EOF {
		list = append(list, p.parseStmt())
	}

	return
}

func (p *parser) parseBody(scope *ast.Scope) *ast.BlockStmt {
	if p.trace {
		defer un(trace(p, "Body"))
	}

	lbrace := p.expect(token.LBRACE)
	p.topScope = scope // open function scope
	list := p.parseStmtList()
	p.closeScope()
	rbrace := p.expect(token.RBRACE)

	return &ast.BlockStmt{Lbrace: lbrace, List: list, Rbrace: rbrace}
}

// ----------------------------------------------------------------------------
// Statements

func (p *parser) parseSimpleStmt() ast.Stmt {
	if p.trace {
		defer un(trace(p, "SimpleStmt"))
	}

	x := p.parseExpr(true)

	switch p.tok {
	case
		token.ASSIGN,
		token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN,
		token.QUO_ASSIGN, token.REM_ASSIGN:
		// assignment statement
		pos, tok := p.pos, p.tok
		p.next()
		y := p.parseExpr(false)
		as := &ast.AssignStmt{Lhs: x, TokPos: pos, Tok: tok, Rhs: y}
		return as
	}

	// expression
	return &ast.ExprStmt{X: x}
}

func (p *parser) parseStmt() (s ast.Stmt) {
	if p.trace {
		defer un(trace(p, "Statement"))
	}

	switch p.tok {
	case
		// tokens that may start an expression
		token.IDENT, token.INT, token.FLOAT, token.STRING, token.LPAREN, // operands
		token.LBRACK, // composite types
		token.NOT:    // unary operators
		s = p.parseSimpleStmt()
		p.expectSemi()
	case token.LBRACE:
		s = p.parseBlockStmt()
		p.expectSemi()
	case token.RETURN:
		s = p.parseReturnStmt()
	case token.IF:
		s = p.parseIfStmt()
	case token.FOR:
		s = p.parseForStmt()
	case token.SEMICOLON:
		// Is it ever possible to have an implicit semicolon
		// producing an empty statement in a valid program?
		// (handle correctly anyway)
		s = &ast.EmptyStmt{Semicolon: p.pos, Implicit: p.lit == "\n"}
		p.next()
	case token.RBRACE:
		// a semicolon may be omitted before a closing "}"
		s = &ast.EmptyStmt{Semicolon: p.pos, Implicit: true}
		p.next()
	default:
		// no statement found
		pos := p.pos
		p.errorExpected(pos, "statement")
		syncStmt(p)
		s = &ast.BadStmt{From: pos, To: p.pos}
	}

	return
}

func (p *parser) parseBlockStmt() *ast.BlockStmt {
	if p.trace {
		defer un(trace(p, "BlockStmt"))
	}

	lbrace := p.expect(token.LBRACE)
	p.openScope()
	list := p.parseStmtList()
	p.closeScope()
	rbrace := p.expect(token.RBRACE)

	return &ast.BlockStmt{Lbrace: lbrace, List: list, Rbrace: rbrace}
}

func (p *parser) parseIfStmt() *ast.IfStmt {
	if p.trace {
		defer un(trace(p, "IfStmt"))
	}

	// "if"
	pos := p.expect(token.IF)

	// An expression (boolean expression)
	prevLev := p.exprLev
	p.exprLev = -1
	x := p.parseExpr(false)
	p.exprLev = prevLev

	// The body block
	body := p.parseBlockStmt()

	// If we have an "else" then we parse the else body
	var else_ ast.Stmt
	if p.tok == token.ELSE {
		p.next()
		switch p.tok {
		case token.IF:
			else_ = p.parseIfStmt()
		case token.LBRACE:
			else_ = p.parseBlockStmt()
			p.expectSemi()
		default:
			p.errorExpected(p.pos, "if statement or block")
			else_ = &ast.BadStmt{From: p.pos, To: p.pos}
		}
	} else {
		p.expectSemi()
	}

	return &ast.IfStmt{If: pos, Cond: x, Body: body, Else: else_}
}

func (p *parser) parseForStmt() *ast.ForStmt {
	if p.trace {
		defer un(trace(p, "ForStmt"))
	}

	// "for"
	pos := p.expect(token.FOR)

	// An expression to loop over
	prevLev := p.exprLev
	p.exprLev = -1
	x := p.parseExpr(false)
	p.exprLev = prevLev

	// "as"
	p.expect(token.AS)

	// Look for the variable name
	name := p.parseIdent()

	// If we have a comma, get the second name
	var name2 *ast.Ident
	if p.tok == token.COMMA {
		p.next()
		name2 = p.parseIdent()
	}

	// The body block
	body := p.parseBlockStmt()

	p.expectSemi()

	return &ast.ForStmt{For: pos, Expr: x, Name: name, Name2: name2, Body: body}
}

func (p *parser) parseReturnStmt() *ast.ReturnStmt {
	if p.trace {
		defer un(trace(p, "ReturnStmt"))
	}

	pos := p.pos
	p.expect(token.RETURN)
	x := p.parseExpr(false)
	p.expectSemi()

	return &ast.ReturnStmt{Return: pos, Result: x}
}

// ----------------------------------------------------------------------------
// Expressions

func (p *parser) parseRule() ast.Expr {
	if p.trace {
		defer un(trace(p, "Rule"))
	}

	pos := p.expect(token.RULE)
	scope := ast.NewScope(p.topScope) // function scope

	p.exprLev++

	lbrace := p.expect(token.LBRACE) // {
	p.topScope = scope               // open rule scope
	expr := p.parseExpr(false)       // parse the one expression
	p.closeScope()

	// Because a rule doesn't contain a statement, it doesn't consume any
	// semicolons at the end. Allow exactly one semicolon (newline).
	if p.tok == token.SEMICOLON {
		p.next()
	}

	rbrace := p.expect(token.RBRACE) // }

	p.exprLev--

	return &ast.RuleLit{Rule: pos, Lbrace: lbrace, Expr: expr, Rbrace: rbrace}
}

func (p *parser) parseFunc() ast.Expr {
	if p.trace {
		defer un(trace(p, "Func"))
	}

	// "func"
	pos := p.expect(token.FUNC)
	scope := ast.NewScope(p.topScope) // function scope

	// expect '('
	lparen := p.expect(token.LPAREN)

	var idents []*ast.Ident
	if p.tok != token.RPAREN {
		for {
			idents = append(idents, p.parseIdent())
			if p.tok != token.COMMA {
				break
			}
			p.next()
			if p.tok == token.RPAREN {
				break
			}
		}
	}

	// expect ')'
	rparen := p.expect(token.RPAREN)

	p.exprLev++
	body := p.parseBody(scope)
	p.exprLev--

	return &ast.FuncLit{
		Func:    pos,
		Opening: lparen,
		Params:  idents,
		Closing: rparen,
		Body:    body,
	}
}

func (p *parser) parseQuantExpr() *ast.QuantExpr {
	if p.trace {
		defer un(trace(p, "QuantExpr"))
	}

	// "any" or "all"
	var pos token.Pos
	op := p.tok
	switch op {
	case token.ANY:
		pos = p.expect(token.ANY)
	case token.ALL:
		pos = p.expect(token.ALL)
	default:
		p.errorExpected(p.pos, "'any' or 'all'")
	}

	// An expression to loop over
	prevLev := p.exprLev
	p.exprLev = -1
	x := p.parseExpr(false)
	p.exprLev = prevLev

	// "as"
	p.expect(token.AS)

	// Look for the variable name
	name := p.parseIdent()

	// If we have a comma, get the second name
	var name2 *ast.Ident
	if p.tok == token.COMMA {
		p.next()
		name2 = p.parseIdent()
	}

	// The body block
	lbrace := p.expect(token.LBRACE) // {
	p.openScope()
	body := p.parseExpr(false) // parse the one expression
	p.closeScope()
	p.expectSemi()
	rbrace := p.expect(token.RBRACE) // }

	return &ast.QuantExpr{
		Op: op, OpPos: pos, Expr: x,
		Name: name, Name2: name2,
		Lbrace: lbrace, Body: body, Rbrace: rbrace,
	}
}

func (p *parser) parseExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "Expr"))
	}

	return p.parseBinaryExpr(lhs, token.LowestPrec+1)
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseBinaryExpr(lhs bool, prec1 int) ast.Expr {
	if p.trace {
		defer un(trace(p, "BinaryExpr"))
	}

	x := p.parseUnaryExpr(lhs)

	// Determine if we're negating an operator
	neg := false
	if p.tok == token.NOTSTR {
		neg = true
		p.next()
	}

	// Find the next operator
	for _, prec := p.tokPrec(); prec >= prec1; prec-- {
		for {
			op, oprec := p.tokPrec()
			if oprec != prec {
				break
			}
			pos := p.expect(op)

			// If we're negating, it must be a specific operator that we find
			if neg {
				switch op {
				case token.CONTAINS, token.MATCHES, token.IN:
					// Okay

				default:
					p.errorExpected(pos, "contains, matches, or in")
				}
			}

			// Special case if we see an IS followed by "not" then the
			// operator becomes ISNOT which has the same precedence as IS
			if op == token.IS && p.tok == token.NOTSTR {
				p.next()
				op = token.ISNOT
			}

			if lhs {
				p.resolve(x)
				lhs = false
			}
			y := p.parseBinaryExpr(false, prec+1)
			x = &ast.BinaryExpr{X: x, OpPos: pos, Op: op, OpNeg: neg, Y: y}
		}
	}

	return x
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseUnaryExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "UnaryExpr"))
	}

	switch p.tok {
	case token.ADD, token.SUB, token.NOT, token.NOTSTR:
		pos, op := p.pos, p.tok
		p.next()
		x := p.parseUnaryExpr(false)
		return &ast.UnaryExpr{OpPos: pos, Op: op, X: x}

	}

	return p.parsePrimaryExpr(lhs)
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parsePrimaryExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "PrimaryExpr"))
	}

	x := p.parseOperand(lhs)
L:
	for {
		switch p.tok {
		case token.PERIOD:
			p.next()
			if lhs {
				p.resolve(x)
			}

			switch p.tok {
			case token.IDENT:
				x = p.parseSelector(x)
			default:
				pos := p.pos
				p.errorExpected(pos, "selector")
				p.next() // make progress
				sel := &ast.Ident{NamePos: pos, Name: "_"}
				x = &ast.SelectorExpr{X: x, Sel: sel}
			}

		case token.LBRACK:
			// a[0], a[0:2], etc.

			if lhs {
				p.resolve(x)
			}

			x = p.parseIndexOrSlice(x)

		case token.LPAREN:
			if lhs {
				p.resolve(x)
			}

			x = p.parseCall(x)
		default:
			break L
		}
		lhs = false // no need to try to resolve again
	}

	return x
}

func (p *parser) parseOperand(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "Operand"))
	}

	switch p.tok {
	case token.IDENT:
		x := p.parseIdent()
		if !lhs {
			p.resolve(x)
		}
		return x

	case token.INT, token.FLOAT, token.STRING:
		x := &ast.BasicLit{ValuePos: p.pos, Kind: p.tok, Value: p.lit}
		p.next()
		return x

	case token.LBRACK:
		x := p.parseList()
		return x

	case token.LBRACE:
		x := p.parseMap()
		return x

	case token.LPAREN:
		lparen := p.pos
		p.next()
		x := p.parseExpr(false)
		rparen := p.expect(token.RPAREN)
		return &ast.ParenExpr{Lparen: lparen, X: x, Rparen: rparen}

	case token.FUNC:
		return p.parseFunc()

	case token.RULE:
		return p.parseRule()

	case token.ANY, token.ALL:
		return p.parseQuantExpr()
	}

	// we have an error
	pos := p.pos
	p.errorExpected(pos, "operand")
	syncStmt(p)
	return &ast.BadExpr{From: pos, To: p.pos}
}

func (p *parser) tokPrec() (token.Token, int) {
	tok := p.tok
	if p.inRhs && tok == token.ASSIGN {
		tok = token.EQL
	}
	return tok, tok.Precedence()
}

func (p *parser) parseList() *ast.ListLit {
	if p.trace {
		defer un(trace(p, "List"))
	}

	// Get the left '['
	lbrack := p.expect(token.LBRACK)

	// If it isn't an empty list, then parse the element list
	var elts []ast.Expr
	if p.tok != token.RBRACK {
		elts = p.parseElementList(false)
	}

	// Get closing brack
	rbrack := p.expectClosing(token.RBRACK, "list literal")

	return &ast.ListLit{
		Lbrack: lbrack,
		Elts:   elts,
		Rbrack: rbrack,
	}
}

func (p *parser) parseMap() *ast.MapLit {
	if p.trace {
		defer un(trace(p, "Map"))
	}

	// Get the left '{'
	lbrace := p.expect(token.LBRACE)

	// If it isn't an empty list, then parse the element list
	var elts []ast.Expr
	if p.tok != token.RBRACE {
		elts = p.parseElementList(true)
	}

	// Get closing brace
	rbrace := p.expectClosing(token.RBRACE, "map literal")

	return &ast.MapLit{
		Lbrace: lbrace,
		Elts:   elts,
		Rbrace: rbrace,
	}
}

func (p *parser) parseElementList(m bool) (list []ast.Expr) {
	if p.trace {
		defer un(trace(p, "ElementList"))
	}

	kind := "list literal"
	end := token.RBRACK
	if m {
		kind = "map literal"
		end = token.RBRACE
	}

	for p.tok != end && p.tok != token.EOF {
		list = append(list, p.parseElement(m))
		if !p.atComma(kind, end) {
			break
		}

		p.next()
	}

	return
}

func (p *parser) parseElement(m bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "Element"))
	}

	x := p.parseExpr(false)

	if m {
		// Map, we should look for a colon
		colon := p.expect(token.COLON)
		x = &ast.KeyValueExpr{Key: x, Colon: colon, Value: p.parseExpr(false)}
	}

	return x
}

func (p *parser) parseSelector(x ast.Expr) ast.Expr {
	if p.trace {
		defer un(trace(p, "Selector"))
	}

	sel := p.parseIdent()
	return &ast.SelectorExpr{X: x, Sel: sel}
}

func (p *parser) parseIndexOrSlice(x ast.Expr) ast.Expr {
	if p.trace {
		defer un(trace(p, "IndexOrSlice"))
	}

	const N = 2 // Number of things allowed in a slice expression

	// Expect [ first
	lbrack := p.expect(token.LBRACK)
	p.exprLev++

	// Go through and build up the indices split by colons. This may
	// just be one if there are no colons.
	var index [N]ast.Expr
	var colons [N - 1]token.Pos
	if p.tok != token.COLON {
		index[0] = p.parseExpr(false)
	}

	// Default to no colons, but while there are colons, then record it.
	ncolons := 0
	for p.tok == token.COLON && ncolons < len(colons) {
		colons[ncolons] = p.pos
		ncolons++
		p.next()
		if p.tok != token.COLON && p.tok != token.RBRACK && p.tok != token.EOF {
			index[ncolons] = p.parseExpr(false)
		}
	}

	// Expect closing ]
	p.exprLev--
	rbrack := p.expect(token.RBRACK)

	// If we had colons, this is a slice.
	if ncolons > 0 {
		return &ast.SliceExpr{
			X:      x,
			Lbrack: lbrack,
			Low:    index[0],
			High:   index[1],
			Rbrack: rbrack,
		}
	}

	// Otherwise, just an index expression
	return &ast.IndexExpr{X: x, Lbrack: lbrack, Index: index[0], Rbrack: rbrack}
}

func (p *parser) parseCall(fun ast.Expr) *ast.CallExpr {
	if p.trace {
		defer un(trace(p, "Call"))
	}

	lparen := p.expect(token.LPAREN)
	p.exprLev++
	var list []ast.Expr
	for p.tok != token.RPAREN && p.tok != token.EOF {
		list = append(list, p.parseExpr(false))
		if !p.atComma("argument list", token.RPAREN) {
			break
		}
		p.next()
	}
	p.exprLev--
	rparen := p.expectClosing(token.RPAREN, "argument list")

	return &ast.CallExpr{Fun: fun, Lparen: lparen, Args: list, Rparen: rparen}
}

// ----------------------------------------------------------------------------
// Declarations

func isValidImport(lit string) bool {
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`\uFFFD"
	s, _ := strconv.Unquote(lit) // go/scanner returns a legal string literal
	for _, r := range s {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
			return false
		}
	}
	return s != ""
}

func (p *parser) parseImportSpec() *ast.ImportSpec {
	if p.trace {
		defer un(trace(p, "ImportSpec"))
	}

	pos := p.pos
	doc := p.leadComment

	// Expect import
	p.expect(token.IMPORT)

	var path string
	if p.tok == token.STRING {
		path = p.lit
		if !isValidImport(path) {
			p.error(pos, "invalid import path: "+path)
		}
		p.next()
	} else {
		p.expect(token.STRING) // use expect() error handling
	}

	var ident *ast.Ident
	if p.tok == token.AS {
		// We are renaming this import under another identifier
		p.next()
		ident = p.parseIdent()
	}

	p.expectSemi() // call before accessing p.linecomment

	// collect imports
	spec := &ast.ImportSpec{
		Doc:     doc,
		Name:    ident,
		Path:    &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: path},
		Comment: p.lineComment,
	}

	return spec
}

// ----------------------------------------------------------------------------
// Source files

func (p *parser) parseFile() *ast.File {
	if p.trace {
		defer un(trace(p, "File"))
	}

	// Don't bother parsing the rest if we had errors scanning the first token.
	// Likely not a source file at all.
	if p.errors.Len() != 0 {
		return nil
	}

	// If we have a first comment take it as the doc if it is early enough.
	var doc *ast.CommentGroup
	if len(p.comments) > 0 {
		temp := p.comments[0]

		// The doc line must be on a line alone by itself. We check here
		// whether we're at least one line away from another token.
		tempLine := p.file.Line(temp.End())
		firstLine := p.file.Line(p.pos)
		if tempLine+1 < firstLine {
			doc = temp
		}
	}

	p.openScope()
	p.pkgScope = p.topScope
	var stmts []ast.Stmt

	// import decls (must appear at the top of the file)
	var imports []*ast.ImportSpec
	for p.tok == token.IMPORT {
		imports = append(imports, p.parseImportSpec())
	}

	// rest of the source file which is a statement list
	for p.tok != token.EOF {
		stmts = append(stmts, p.parseStmt())
	}

	p.closeScope()
	assert(p.topScope == nil, "unbalanced scopes")

	// resolve global identifiers within the same file
	i := 0
	for _, ident := range p.unresolved {
		// i <= index for current ident
		assert(ident.Obj == unresolved, "object already resolved")
		ident.Obj = p.pkgScope.Lookup(ident.Name) // also removes unresolved sentinel
		if ident.Obj == nil {
			p.unresolved[i] = ident
			i++
		}
	}

	return &ast.File{
		Doc:        doc,
		Scope:      p.pkgScope,
		Imports:    imports,
		Stmts:      stmts,
		Unresolved: p.unresolved[0:i],
		Comments:   p.comments,
	}
}

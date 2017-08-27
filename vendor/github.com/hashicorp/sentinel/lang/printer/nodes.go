// Portions of this file are taken directly from the Go source code
// licensed under the BSD 3-clause license. These lines are copyright
// 2009 The Go Authors.

package printer

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

func (p *printer) file(n *ast.File) {
	if len(n.Imports) > 0 {
		p.importList(n.Imports)
	}

	p.stmtList(n.Stmts, 0)
}

//-------------------------------------------------------------------
// Imports

func (p *printer) importList(impts []*ast.ImportSpec) {
	var line int
	for i, impt := range impts {
		if len(p.output) > 0 {
			// only print line break if we are not at the beginning of the output
			// (i.e., we are not printing only a partial program)
			p.linebreak(p.lineFor(impt.Pos()), 1, ignore, i == 0 || p.linesFrom(line) > 0)
		}

		p.recordLine(&line)
		p.importSpec(impt)
	}
}

func (p *printer) importSpec(impt *ast.ImportSpec) {
	p.setComment(impt.Doc)
	p.print(impt.Pos(), token.IMPORT, blank)
	p.expr(impt.Path)
}

//-------------------------------------------------------------------
// Expressions

func (p *printer) expr(expr ast.Expr) {
	p.expr1(expr, token.LowestPrec, 1)
}

func (p *printer) expr0(expr ast.Expr, depth int) {
	p.expr1(expr, token.LowestPrec, depth)
}

func (p *printer) expr1(expr ast.Expr, prec, depth int) {
	p.print(expr.Pos())

	switch x := expr.(type) {
	case *ast.Ident:
		p.print(x)

	case *ast.BasicLit:
		p.print(x)

	case *ast.FuncLit:
		p.print(token.FUNC)
		p.print(x.Opening, token.LPAREN)
		for i, param := range x.Params {
			if i > 0 {
				p.print(token.COMMA, blank)
			}

			p.expr(param)
		}
		p.print(x.Closing, token.RPAREN)
		p.funcBody(p.distanceFrom(x.Func), x.Body)

	case *ast.ListLit:
		p.print(x.Lbrack, token.LBRACK)
		p.elts(x.Lbrack, x.Elts, x.Rbrack)
		p.print(x.Rbrack, token.RBRACK)

	case *ast.MapLit:
		p.print(x.Lbrace, token.LBRACE)
		p.elts(x.Lbrace, x.Elts, x.Rbrace)
		p.print(x.Rbrace, token.RBRACE)

	case *ast.QuantExpr:
		p.quant(x)

	case *ast.UnaryExpr:
		p.print(x.Op)
		if x.Op == token.NOTSTR {
			p.print(blank)
		}

		p.expr(x.X)

	case *ast.BinaryExpr:
		p.binaryExpr(x, prec, depth)

	case *ast.ParenExpr:
		p.print(token.LPAREN)
		p.expr(x.X)
		p.print(x.Rparen, token.RPAREN)

	case *ast.SelectorExpr:
		p.selector(x)

	case *ast.IndexExpr:
		p.expr(x.X)
		p.print(x.Lbrack, token.LBRACK)
		p.expr(x.Index)
		p.print(x.Rbrack, token.RBRACK)

	case *ast.SliceExpr:
		p.expr(x.X)
		p.print(x.Lbrack, token.LBRACK)
		indices := []ast.Expr{x.Low, x.High}
		for i, y := range indices {
			if i > 0 {
				// blanks around ":" if both sides exist and either side is a binary expression
				x := indices[i-1]
				if x != nil && y != nil && (isBinary(x) || isBinary(y)) {
					p.print(blank, token.COLON, blank)
				} else {
					p.print(token.COLON)
				}
			}
			if y != nil {
				p.expr(y)
			}
		}
		p.print(x.Rbrack, token.RBRACK)

	case *ast.RuleLit:
		p.rule(x, depth)

	case *ast.CallExpr:
		wasIndented := p.possibleSelector(x.Fun)
		p.print(x.Lparen, token.LPAREN)
		p.elts(x.Lparen, x.Args, x.Rparen)
		p.print(x.Rparen, token.RPAREN)
		if wasIndented {
			p.print(unindent)
		}

	case *ast.KeyValueExpr:
		p.expr(x.Key)
		p.print(x.Colon, token.COLON, blank)
		p.expr(x.Value)

	case Printable:
		n := x.PrintNode()
		if expr, ok := n.(ast.Expr); ok {
			p.expr1(expr, prec, depth)
			return
		}

		panic(fmt.Sprintf("unsupported expr type: %T", expr))

	default:
		panic(fmt.Sprintf("unsupported expr type: %T", expr))
	}
}

func (p *printer) funcBody(headerSize int, b *ast.BlockStmt) {
	// If the body looks "small enough" then put it all on one line
	const maxSize = 100
	if headerSize+p.bodySize(b, maxSize) <= maxSize {
		p.print(blank, b.Lbrace, token.LBRACE)
		if len(b.List) > 0 {
			p.print(blank)
			for i, s := range b.List {
				if i > 0 {
					p.print(token.SEMICOLON, blank)
				}
				p.stmt(s)
			}
			p.print(blank)
		}
		p.print(b.Rbrace, token.RBRACE)

		return
	}

	p.print(blank)
	p.block(b, 1)
}

func (p *printer) elts(prev0 token.Pos, list []ast.Expr, next0 token.Pos) {
	if len(list) == 0 {
		return
	}

	prev := p.posFor(prev0)
	next := p.posFor(next0)
	line := p.lineFor(list[0].Pos())
	endLine := p.lineFor(list[len(list)-1].End())

	if prev.IsValid() && prev.Line == line && line == endLine {
		// all list entries on a single line
		for i, x := range list {
			if i > 0 {
				// use position of expression following the comma as
				// comma position for correct comment placement
				p.print(x.Pos(), token.COMMA, blank)
			}
			p.expr(x)
		}
		return
	}

	// list entries span multiple lines;
	// use source code positions to guide line breaks

	// don't add extra indentation if noIndent is set;
	// i.e., pretend that the first line is already indented
	ws := indent

	// the first linebreak is always a formfeed since this section must not
	// depend on any previous formatting
	prevBreak := -1 // index of last expression that was followed by a linebreak
	if prev.IsValid() && prev.Line < line && p.linebreak(line, 0, ws, true) {
		ws = ignore
		prevBreak = 0
	}

	// initialize expression/key size: a zero value indicates expr/key doesn't fit on a single line
	size := 0

	// print all list elements
	prevLine := prev.Line
	for i, x := range list {
		line = p.lineFor(x.Pos())

		// determine if the next linebreak, if any, needs to use formfeed:
		// in general, use the entire node size to make the decision; for
		// key:value expressions, use the key size
		// TODO(gri) for a better result, should probably incorporate both
		//           the key and the node size into the decision process
		useFF := true

		// determine element size: all bets are off if we don't have
		// position information for the previous and next token (likely
		// generated code - simply ignore the size in this case by setting
		// it to 0)
		prevSize := size
		const infinity = 1e6 // larger than any source line
		size = p.nodeSize(x, infinity)
		pair, isPair := x.(*ast.KeyValueExpr)
		if size <= infinity && prev.IsValid() && next.IsValid() {
			// x fits on a single line
			if isPair {
				size = p.nodeSize(pair.Key, infinity) // size <= infinity
			}
		} else {
			// size too large or we don't have good layout information
			size = 0
		}

		// if the previous line and the current line had single-
		// line-expressions and the key sizes are small or the
		// the ratio between the key sizes does not exceed a
		// threshold, align columns and do not use formfeed
		if prevSize > 0 && size > 0 {
			const smallSize = 20
			if prevSize <= smallSize && size <= smallSize {
				useFF = false
			} else {
				const r = 4 // threshold
				ratio := float64(size) / float64(prevSize)
				useFF = ratio <= 1.0/r || r <= ratio
			}
		}

		needsLinebreak := 0 < prevLine && prevLine < line
		if i > 0 {
			// use position of expression following the comma as
			// comma position for correct comment placement, but
			// only if the expression is on the same line
			if !needsLinebreak {
				p.print(x.Pos())
			}
			p.print(token.COMMA)
			needsBlank := true
			if needsLinebreak {
				// lines are broken using newlines so comments remain aligned
				// unless forceFF is set or there are multiple expressions on
				// the same line in which case formfeed is used
				if p.linebreak(line, 0, ws, useFF || prevBreak+1 < i) {
					ws = ignore
					prevBreak = i
					needsBlank = false // we got a line break instead
				}
			}
			if needsBlank {
				p.print(blank)
			}
		}

		if len(list) > 1 && isPair && size > 0 && needsLinebreak {
			// we have a key:value expression that fits onto one line
			// and it's not on the same line as the prior expression:
			// use a column for the key such that consecutive entries
			// can align if possible
			// (needsLinebreak is set if we started a new line before)
			p.expr(pair.Key)
			p.print(pair.Colon, token.COLON, vtab)
			p.expr(pair.Value)
		} else {
			p.expr(x)
		}

		prevLine = line
	}

	if next.IsValid() && p.pos.Line < next.Line {
		// print a terminating comma if the next token is on a new line
		p.print(token.COMMA)
		if ws == ignore {
			// unindent if we indented
			p.print(unindent)
		}
		p.print(formfeed) // terminating comma needs a line break to look good
		return
	}

	if ws == ignore {
		// unindent if we indented
		p.print(unindent)
	}

}

func (p *printer) quant(x *ast.QuantExpr) {
	// Print the expression and assignments. This part is straightforward.
	p.print(x.Op, blank)
	p.expr(x.Expr)
	p.print(blank, token.AS, blank)
	p.expr(x.Name)
	if x.Name2 != nil {
		p.print(token.COMMA, blank)
		p.expr(x.Name2)
	}

	p.print(blank, x.Lbrace, token.LBRACE)

	// If the body looks "small enough" then put it all on one line. We
	// calculate small enough by simulating a body.
	const maxSize = 100
	headerSize := p.distanceFrom(x.OpPos)
	multiline := headerSize+p.exprBodySize(x.Lbrace, x.Rbrace, x.Body, maxSize) > maxSize

	// Perform the body printing. If we're multiline then we indent and
	// match linebreaks. Otherwise, we just put blanks around the body.

	if multiline {
		p.print(indent)
		p.linebreak(p.lineFor(x.Body.Pos()), 1, ignore, true)
	} else {
		p.print(blank)
	}

	p.expr(x.Body)

	if multiline {
		p.print(unindent)
		p.linebreak(p.lineFor(x.Rbrace), 1, ignore, true)
	} else {
		p.print(blank)
	}

	p.print(x.Rbrace, token.RBRACE)
}

func (p *printer) binaryExpr(x *ast.BinaryExpr, prec, depth int) {
	xline := p.pos.Line // before the operator (it may be on the next line!)
	yline := p.lineFor(x.Y.Pos())

	// Print the LHS + Op, always easy. Only sneaky part is checking whether
	// this is a negated operation, in which case we write the not.
	p.expr1(x.X, prec, depth)
	if x.OpNeg {
		p.print(blank, x.OpPos, token.NOTSTR)
	}
	p.print(blank, x.OpPos, x.Op)

	// Next, determine if we're putting the RHS on another line.
	ws := indent
	if depth > 1 {
		ws = ignore
	}

	printBlank := true
	if xline != yline && xline > 0 && yline > 0 {
		// If there are no comments before the next line, then we
		// put the clause directly on the next line.
		if !p.commentBefore(p.posFor(x.Y.Pos())) {
			yline = xline + 1
		}

		// at least one line break, but respect an extra empty line
		// in the source
		if p.linebreak(yline, 1, ws, true) {
			ws = ignore
			printBlank = false // no blank after line break
		}
	}
	if printBlank {
		p.print(blank)
	}

	// Finally, RHS
	p.expr1(x.Y, prec, depth)
}

func (p *printer) possibleSelector(expr ast.Expr) bool {
	if x, ok := expr.(*ast.SelectorExpr); ok {
		return p.selector(x)
	}

	p.expr(expr)
	return false
}

func (p *printer) selector(x *ast.SelectorExpr) bool {
	p.expr(x.X)
	p.print(token.PERIOD)

	// If the selector is on another line, then we format by indenting
	if line := p.lineFor(x.Sel.Pos()); p.pos.IsValid() && p.pos.Line < line {
		p.print(indent, newline, x.Sel.Pos(), x.Sel, unindent)
		return true
	}

	// Same line, just output selector
	p.print(x.Sel.Pos(), x.Sel)
	return false
}

func (p *printer) rule(x *ast.RuleLit, depth int) {
	p.print(x.Rule, token.RULE)
	p.print(blank, x.Lbrace, token.LBRACE)

	// If the body looks "small enough" then put it all on one line. We
	// calculate small enough by simulating a body.
	const maxSize = 100
	headerSize := p.distanceFrom(x.Rule)
	multiline := headerSize+p.exprBodySize(x.Lbrace, x.Rbrace, x.Expr, maxSize) > maxSize

	// Perform the body printing. If we're multiline then we indent and
	// match linebreaks. Otherwise, we just put blanks around the body.

	if multiline {
		// We want the expression to start at exactly the line after
		// the rule opening since we're multiline.
		line := p.lineFor(x.Rule) + 1
		p.print(indent)
		p.linebreak(line, 1, ignore, true)
	} else {
		p.print(blank)
	}

	p.expr0(x.Expr, depth+1)

	if multiline {
		p.print(unindent)
		p.print(newline)
	} else {
		p.print(blank)
	}

	p.print(x.Rbrace, token.RBRACE)
}

//-------------------------------------------------------------------
// Statements

func (p *printer) stmtList(list []ast.Stmt, nindent int) {
	if nindent > 0 {
		p.print(indent)
	}

	var line int
	i := 0
	for _, s := range list {
		// ignore empty statements
		if _, isEmpty := s.(*ast.EmptyStmt); isEmpty {
			continue
		}

		// nindent == 0 only for lists of switch/select case clauses;
		// in those cases each clause is a new section
		if len(p.output) > 0 {
			// only print line break if we are not at the beginning of the output
			// (i.e., we are not printing only a partial program)
			p.linebreak(p.lineFor(s.Pos()), 1, ignore, i == 0 || p.linesFrom(line) > 0)
		}

		p.recordLine(&line)
		p.stmt(s)
		i++
	}

	if nindent > 0 {
		p.print(unindent)
	}
}

func (p *printer) stmt(stmt ast.Stmt) {
	p.print(stmt.Pos())

	switch x := stmt.(type) {
	case *ast.EmptyStmt:
		// Do nothing

	case *ast.AssignStmt:
		p.expr(x.Lhs)
		p.print(blank, x.TokPos, x.Tok, blank)
		p.expr(x.Rhs)

	case *ast.BlockStmt:
		p.block(x, 1)

	case *ast.IfStmt:
		p.print(token.IF)
		p.print(blank)
		p.expr(x.Cond)
		p.print(blank)
		p.block(x.Body, 1)
		if x.Else != nil {
			p.print(blank, token.ELSE, blank)
			p.stmt(x.Else)
		}

	case *ast.ForStmt:
		p.print(token.FOR, blank)
		p.expr(x.Expr)
		p.print(blank, token.AS, blank)
		p.expr(x.Name)
		if x.Name2 != nil {
			p.print(token.COMMA, blank)
			p.expr(x.Name2)
		}
		p.print(blank)
		p.block(x.Body, 1)

	case *ast.ReturnStmt:
		p.print(token.RETURN, blank)
		p.expr(x.Result)

	case *ast.ExprStmt:
		p.expr(x.X)

	default:
		panic(fmt.Sprintf("unsupported stmt type: %T", stmt))
	}
}

// block prints an *ast.BlockStmt; it always spans at least two lines.
func (p *printer) block(b *ast.BlockStmt, nindent int) {
	p.print(b.Lbrace, token.LBRACE)
	p.stmtList(b.List, nindent)
	p.linebreak(p.lineFor(b.Rbrace), 1, ignore, true)
	p.print(b.Rbrace, token.RBRACE)
}

//-------------------------------------------------------------------
// Helpers

// distanceFrom returns the column difference between from and p.pos (the current
// estimated position) if both are on the same line; if they are on different lines
// (or unknown) the result is infinity.
func (p *printer) distanceFrom(from token.Pos) int {
	if from.IsValid() && p.pos.IsValid() {
		if f := p.posFor(from); f.Line == p.pos.Line {
			return p.pos.Column - f.Column
		}
	}
	return infinity
}

// Print as many newlines as necessary (but at least min newlines) to get to
// the current line. ws is printed before the first line break. If newSection
// is set, the first line break is printed as formfeed. Returns true if any
// line break was printed; returns false otherwise.
//
// TODO(gri): linebreak may add too many lines if the next statement at "line"
//            is preceded by comments because the computation of n assumes
//            the current position before the comment and the target position
//            after the comment. Thus, after interspersing such comments, the
//            space taken up by them is not considered to reduce the number of
//            linebreaks. At the moment there is no easy way to know about
//            future (not yet interspersed) comments in this function.
//
func (p *printer) linebreak(line, min int, ws whiteSpace, newSection bool) (printedBreak bool) {
	n := nlimit(line - p.pos.Line)
	if n < min {
		n = min
	}
	if n > 0 {
		p.print(ws)
		if newSection {
			p.print(formfeed)
			n--
		}
		for ; n > 0; n-- {
			p.print(newline)
		}
		printedBreak = true
	}
	return
}

// nodeSize determines the size of n in chars after formatting.
// The result is <= maxSize if the node fits on one line with at
// most maxSize chars and the formatted output doesn't contain
// any control chars. Otherwise, the result is > maxSize.
//
func (p *printer) nodeSize(n ast.Node, maxSize int) (size int) {
	// nodeSize invokes the printer, which may invoke nodeSize
	// recursively. For deep composite literal nests, this can
	// lead to an exponential algorithm. Remember previous
	// results to prune the recursion (was issue 1628).
	if size, found := p.nodeSizes[n]; found {
		return size
	}

	size = maxSize + 1 // assume n doesn't fit
	p.nodeSizes[n] = size

	var buf bytes.Buffer
	if err := Fprint(&buf, p.fset, n); err != nil {
		return
	}
	if buf.Len() <= maxSize {
		for _, ch := range buf.Bytes() {
			if ch < ' ' {
				return
			}
		}
		size = buf.Len() // n fits
		p.nodeSizes[n] = size
	}
	return
}

// bodySize is like nodeSize but it is specialized for *ast.BlockStmt's.
func (p *printer) bodySize(b *ast.BlockStmt, maxSize int) int {
	pos1 := b.Pos()
	pos2 := b.Rbrace
	if pos1.IsValid() && pos2.IsValid() && p.lineFor(pos1) != p.lineFor(pos2) {
		// opening and closing brace are on different lines - don't make it a one-liner
		return maxSize + 1
	}
	if len(b.List) > 5 {
		// too many statements - don't make it a one-liner
		return maxSize + 1
	}
	// otherwise, estimate body size
	bodySize := p.commentSizeBefore(p.posFor(pos2))
	for i, s := range b.List {
		if bodySize > maxSize {
			break // no need to continue
		}
		if i > 0 {
			bodySize += 2 // space for a semicolon and blank
		}
		bodySize += p.nodeSize(s, maxSize)
	}
	return bodySize
}

func (p *printer) exprBodySize(lbrace, rbrace token.Pos, x ast.Expr, maxSize int) int {
	bodyBlock := &ast.BlockStmt{
		Lbrace: lbrace,
		Rbrace: rbrace,
		List:   []ast.Stmt{&ast.ExprStmt{X: x}},
	}

	return p.bodySize(bodyBlock, maxSize)
}

func isBinary(expr ast.Expr) bool {
	_, ok := expr.(*ast.BinaryExpr)
	return ok
}

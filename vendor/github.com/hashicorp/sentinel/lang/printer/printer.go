// Portions of this file are taken directly from the Go source code
// licensed under the BSD 3-clause license. These lines are copyright
// 2009 The Go Authors.

// Package printer implements printing of Sentinel AST nodes.
package printer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

const (
	maxNewlines = 2     // max. number of newlines between source text
	debug       = false // enable for debugging
	infinity    = 1 << 30
)

type whiteSpace byte

const (
	ignore   = whiteSpace(0)
	blank    = whiteSpace(' ')
	vtab     = whiteSpace('\v')
	newline  = whiteSpace('\n')
	formfeed = whiteSpace('\f')
	indent   = whiteSpace('>')
	unindent = whiteSpace('<')
)

// commentInfo keeps track of the metadata about a comment while printing.
//
// Note: the field names in this struct are purposely prefixed with "c"
// (rather than just being "idx" for example) because we access them as
// top-level structs when embedded in printer.
type commentInfo struct {
	cindex   int               // comment index
	comment  *ast.CommentGroup // the actual comment
	coffset  int               // posFor(c.Pos()).Offset; or infinity
	cnewline bool              // true if the comment group contains newlines
}

type printer struct {
	fset *token.FileSet

	output      []byte       // accumulated printer result
	impliedSemi bool         // if set, a linebreak implies a semicolon
	indent      int          /// current indentation
	lastTok     token.Token  // last token printed (token.ILLEGAL if it's whitespace)
	wsbuf       []whiteSpace // delayed whitespace

	// Position tracking.
	pos     token.Position
	out     token.Position
	last    token.Position
	linePtr *int // if set, records out.line for the next token here

	// Comment tracking. comments contains the list of upcoming comments
	// and commentInfo contains the information for the next comment.
	comments           []*ast.CommentGroup
	ignoreNodeComments bool // if set, ignore lead and line comments of nodes
	commentInfo

	// Cache of already computed node sizes
	nodeSizes map[ast.Node]int
}

func newPrinter() *printer {
	// Allocate printer
	p := &printer{
		// Allocate whitespace buffer, we never really have more than 16 straight
		wsbuf: make([]whiteSpace, 0, 16),

		// Set initial positional information for output
		pos: token.Position{Line: 1, Column: 1},
		out: token.Position{Line: 1, Column: 1},

		// Allocate node size cache
		nodeSizes: make(map[ast.Node]int),
	}

	return p
}

func (p *printer) print(args ...interface{}) {
	for _, arg := range args {
		var data string
		var impliedSemi, isLit bool

		switch x := arg.(type) {
		case whiteSpace:
			if x == ignore {
				// don't add ignore's to the buffer; they
				// may screw up "correcting" unindents (see
				// LabeledStmt)
				continue
			}

			i := len(p.wsbuf)
			if i == cap(p.wsbuf) {
				// Whitespace sequences are very short so this should
				// never happen. Handle gracefully (but possibly with
				// bad comment placement) if it does happen.
				p.writeWhitespace(i)
				i = 0
			}

			p.wsbuf = p.wsbuf[0 : i+1]
			p.wsbuf[i] = x

			if x == newline || x == formfeed {
				// newlines affect the current state (p.impliedSemi)
				// and not the state after printing arg (impliedSemi)
				// because comments can be interspersed before the arg
				// in this case
				p.impliedSemi = false
			}

			p.lastTok = token.ILLEGAL
			continue

		case token.Pos:
			if x.IsValid() {
				p.pos = p.posFor(x) // accurate position of next item
			}

			continue

		case token.Token:
			data = x.String()
			p.lastTok = x

		case *ast.Ident:
			data = x.Name
			impliedSemi = true
			p.lastTok = token.IDENT

		case *ast.BasicLit:
			data = x.Value
			isLit = true
			impliedSemi = true
			p.lastTok = x.Kind

		default:
			panic(fmt.Sprintf("print: unsupported argument %v (%T)\n", arg, arg))
		}

		next := p.pos // estimated/accurate position of next item
		wroteNewline, droppedFF := p.flush(next, p.lastTok)

		// intersperse extra newlines if present in the source and
		// if they don't cause extra semicolons (don't do this in
		// flush as it will cause extra newlines at the end of a file)
		if !p.impliedSemi {
			n := nlimit(next.Line - p.pos.Line)

			// don't exceed maxNewlines if we already wrote one
			if wroteNewline && n == maxNewlines {
				n = maxNewlines - 1
			}
			if n > 0 {
				ch := byte('\n')
				if droppedFF {
					ch = '\f' // use formfeed since we dropped one before
				}
				p.writeByte(ch, n)
				impliedSemi = false
			}
		}

		// the next token starts now - record its line number if requested
		if p.linePtr != nil {
			*p.linePtr = p.out.Line
			p.linePtr = nil
		}

		p.writeWhitespace(len(p.wsbuf))
		p.writeString(next, data, isLit)
		p.impliedSemi = impliedSemi
	}
}

// printNode pretty prints the given node. Node must be an ast.Node.
// The pretty-printed output is appended to the output of this printer.
func (p *printer) printNode(node interface{}) error {
	// If we are printing a file, then setup the global comment scope
	if x, ok := node.(*ast.File); ok && len(x.Comments) > 0 {
		p.comments = make([]*ast.CommentGroup, len(x.Comments))
		copy(p.comments, x.Comments)
	}

	// If we set some comments, ignore the comments that are directly
	// on the nodes themselves since we have a full list in the file.
	p.ignoreNodeComments = p.comments != nil

	// Prepare comments
	p.nextComment()

	switch x := node.(type) {
	case ast.Expr:
		p.expr(x)

	case ast.Stmt:
		p.stmt(x)

	case *ast.File:
		p.file(x)

		// Flush outstanding comments.
		p.impliedSemi = false // EOF doesn't imply semicolon
		p.flush(token.Position{Offset: infinity, Line: infinity}, token.EOF)

	default:
		return fmt.Errorf("sentinel/lang/printer: unsupported node type %T", node)
	}

	return nil
}

//-------------------------------------------------------------------
// Writers

// flush prints any pending comments and whitespace occurring textually
// before the position of the next token tok. The flush result indicates
// if a newline was written or if a formfeed was dropped from the whitespace
// buffer.
//
func (p *printer) flush(next token.Position, tok token.Token) (wroteNewline, droppedFF bool) {
	if p.commentBefore(next) {
		// if there are comments before the next item, intersperse them
		wroteNewline, droppedFF = p.intersperseComments(next, tok)
	} else {
		// otherwise, write any leftover whitespace
		p.writeWhitespace(len(p.wsbuf))
	}

	return
}

// writeByte writes ch n times to p.output and updates p.pos.
func (p *printer) writeByte(ch byte, n int) {
	if p.out.Column == 1 {
		p.atLineBegin(p.pos)
	}

	for i := 0; i < n; i++ {
		p.output = append(p.output, ch)
	}

	// update positions
	p.pos.Offset += n
	if ch == '\n' || ch == '\f' {
		p.pos.Line += n
		p.out.Line += n
		p.pos.Column = 1
		p.out.Column = 1
		return
	}
	p.pos.Column += n
	p.out.Column += n
}

// writeString writes the string s to p.output and updates p.pos, p.out,
// and p.last. If isLit is set, s is escaped w/ tabwriter.Escape characters
// to protect s from being interpreted by the tabwriter.
//
// Note: writeString is only used to write Go tokens, literals, and
// comments, all of which must be written literally. Thus, it is correct
// to always set isLit = true. However, setting it explicitly only when
// needed (i.e., when we don't know that s contains no tabs or line breaks)
// avoids processing extra escape characters and reduces run time of the
// printer benchmark by up to 10%.
//
func (p *printer) writeString(pos token.Position, s string, isLit bool) {
	if p.out.Column == 1 {
		p.atLineBegin(pos)
	}

	if pos.IsValid() {
		// update p.pos (if pos is invalid, continue with existing p.pos)
		// Note: Must do this after handling line beginnings because
		// atLineBegin updates p.pos if there's indentation, but p.pos
		// is the position of s.
		p.pos = pos
	}

	/*
		if isLit {
			// Protect s such that is passes through the tabwriter
			// unchanged. Note that valid Go programs cannot contain
			// tabwriter.Escape bytes since they do not appear in legal
			// UTF-8 sequences.
			p.output = append(p.output, tabwriter.Escape)
		}
	*/

	p.output = append(p.output, s...)

	// Update positions
	nlines := 0
	var li int // index of last newline; valid if nlines > 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			nlines++
			li = i
		}
	}
	p.pos.Offset += len(s) // Offset just moves by byte length of s
	if nlines > 0 {
		// If newlines exist, then we need to add the newlines to the
		// position and calculate the new column.
		p.pos.Line += nlines
		p.out.Line += nlines

		// Column calculation is just the length of s minus the index of
		// the final newline found.
		c := len(s) - li
		p.pos.Column = c
		p.out.Column = c
	} else {
		// No newlines were added, meaning the column just changes by len(s)
		p.pos.Column += len(s)
		p.out.Column += len(s)
	}

	/*
		if isLit {
			p.output = append(p.output, tabwriter.Escape)
		}
	*/

	p.last = p.pos
}

// whiteWhitespace writes the first n whitespace entries.
func (p *printer) writeWhitespace(n int) {
	// write entries
	for i := 0; i < n; i++ {
		switch ch := p.wsbuf[i]; ch {
		case ignore:
			// ignore!

		case indent:
			p.indent++

		case unindent:
			p.indent--
			if p.indent < 0 {
				panic(fmt.Sprintf("negative indentation:", p.indent))
			}

		case newline, formfeed:
			fallthrough

		default:
			p.writeByte(byte(ch), 1)
		}
	}

	// shift remaining entries down
	l := copy(p.wsbuf, p.wsbuf[n:])
	p.wsbuf = p.wsbuf[:l]
}

// atLineBegin is called when emitting a new line beginning
func (p *printer) atLineBegin(pos token.Position) {
	// write indentation
	// use "hard" htabs - indentation columns
	// must not be discarded by the tabwriter
	n := p.indent // include base indentation
	for i := 0; i < n; i++ {
		p.output = append(p.output, '\t')
	}

	// update positions
	p.pos.Offset += n
	p.pos.Column += n
	p.out.Column += n
}

// writeCommentPrefix writes the whitespace before a comment.
// If there is any pending whitespace, it consumes as much of
// it as is likely to help position the comment nicely.
// pos is the comment position, next the position of the item
// after all pending comments, prev is the previous comment in
// a group of comments (or nil), and tok is the next token.
//
func (p *printer) writeCommentPrefix(pos, next token.Position, prev, comment *ast.Comment, tok token.Token) {
	if len(p.output) == 0 {
		// the comment is the first item to be printed - don't write any whitespace
		return
	}

	if pos.Line == p.last.Line && (prev == nil || prev.Text[1] != '/') {
		// comment on the same line as last item:
		// separate with at least one separator
		hasSep := false
		if prev == nil {
			// first comment of a comment group
			j := 0
			for i, ch := range p.wsbuf {
				switch ch {
				case blank:
					// ignore any blanks before a comment
					p.wsbuf[i] = ignore
					continue
				case vtab:
					// respect existing tabs - important
					// for proper formatting of commented structs
					hasSep = true
					continue
				case indent:
					// apply pending indentation
					continue
				}
				j = i
				break
			}
			p.writeWhitespace(j)
		}
		// make sure there is at least one separator
		if !hasSep {
			sep := byte('\t')
			if pos.Line == next.Line {
				// next item is on the same line as the comment
				// (which must be a /*-style comment): separate
				// with a blank instead of a tab
				sep = ' '
			}
			p.writeByte(sep, 1)
		}

	} else {
		// comment on a different line:
		// separate with at least one line break
		droppedLinebreak := false
		j := 0
		for i, ch := range p.wsbuf {
			switch ch {
			case blank, vtab:
				// ignore any horizontal whitespace before line breaks
				p.wsbuf[i] = ignore
				continue
			case indent:
				// apply pending indentation
				continue
			case unindent:
				// if this is not the last unindent, apply it
				// as it is (likely) belonging to the last
				// construct (e.g., a multi-line expression list)
				// and is not part of closing a block
				if i+1 < len(p.wsbuf) && p.wsbuf[i+1] == unindent {
					continue
				}
				// if the next token is not a closing }, apply the unindent
				// if it appears that the comment is aligned with the
				// token; otherwise assume the unindent is part of a
				// closing block and stop (this scenario appears with
				// comments before a case label where the comments
				// apply to the next case instead of the current one)
				if tok != token.RBRACE && pos.Column == next.Column {
					continue
				}
			case newline, formfeed:
				p.wsbuf[i] = ignore
				droppedLinebreak = prev == nil // record only if first comment of a group
			}
			j = i
			break
		}
		p.writeWhitespace(j)

		// determine number of linebreaks before the comment
		n := 0
		if pos.IsValid() && p.last.IsValid() {
			n = pos.Line - p.last.Line
			if n < 0 { // should never happen
				n = 0
			}
		}

		// at the package scope level only (p.indent == 0),
		// add an extra newline if we dropped one before:
		// this preserves a blank line before documentation
		// comments at the package scope level (issue 2570)
		if p.indent == 0 && droppedLinebreak {
			n++
		}

		// make sure there is at least one line break
		// if the previous comment was a line comment
		if n == 0 && prev != nil && prev.Text[1] == '/' {
			n = 1
		}

		if n > 0 {
			// use formfeeds to break columns before a comment;
			// this is analogous to using formfeeds to separate
			// individual lines of /*-style comments
			p.writeByte('\f', nlimit(n))
		}
	}
}

func (p *printer) writeComment(comment *ast.Comment) {
	text := comment.Text
	pos := p.posFor(comment.Pos())

	const linePrefix = "//line "
	if strings.HasPrefix(text, linePrefix) && (!pos.IsValid() || pos.Column == 1) {
		// possibly a line directive
		ldir := strings.TrimSpace(text[len(linePrefix):])
		if i := strings.LastIndex(ldir, ":"); i >= 0 {
			if line, err := strconv.Atoi(ldir[i+1:]); err == nil && line > 0 {
				// The line directive we are about to print changed
				// the Filename and Line number used for subsequent
				// tokens. We have to update our AST-space position
				// accordingly and suspend indentation temporarily.
				indent := p.indent
				p.indent = 0
				defer func() {
					p.pos.Filename = ldir[:i]
					p.pos.Line = line
					p.pos.Column = 1
					p.indent = indent
				}()
			}
		}
	}

	// shortcut common case of //-style comments
	if text[1] == '/' {
		p.writeString(pos, trimRight(text), true)
		return
	}

	// for /*-style comments, print line by line and let the
	// write function take care of the proper indentation
	lines := strings.Split(text, "\n")

	// The comment started in the first column but is going
	// to be indented. For an idempotent result, add indentation
	// to all lines such that they look like they were indented
	// before - this will make sure the common prefix computation
	// is the same independent of how many times formatting is
	// applied (was issue 1835).
	if pos.IsValid() && pos.Column == 1 && p.indent > 0 {
		for i, line := range lines[1:] {
			lines[1+i] = "   " + line
		}
	}

	stripCommonPrefix(lines)

	// write comment lines, separated by formfeed,
	// without a line break after the last line
	for i, line := range lines {
		if i > 0 {
			p.writeByte('\f', 1)
			pos = p.pos
		}
		if len(line) > 0 {
			p.writeString(pos, trimRight(line), true)
		}
	}
}

// writeCommentSuffix writes a line break after a comment if indicated
// and processes any leftover indentation information. If a line break
// is needed, the kind of break (newline vs formfeed) depends on the
// pending whitespace. The writeCommentSuffix result indicates if a
// newline was written or if a formfeed was dropped from the whitespace
// buffer.
//
func (p *printer) writeCommentSuffix(needsLinebreak bool) (wroteNewline, droppedFF bool) {
	for i, ch := range p.wsbuf {
		switch ch {
		case blank, vtab:
			// ignore trailing whitespace
			p.wsbuf[i] = ignore
		case indent, unindent:
			// don't lose indentation information
		case newline, formfeed:
			// if we need a line break, keep exactly one
			// but remember if we dropped any formfeeds
			if needsLinebreak {
				needsLinebreak = false
				wroteNewline = true
			} else {
				if ch == formfeed {
					droppedFF = true
				}
				p.wsbuf[i] = ignore
			}
		}
	}
	p.writeWhitespace(len(p.wsbuf))

	// make sure we have a line break
	if needsLinebreak {
		p.writeByte('\n', 1)
		wroteNewline = true
	}

	return
}

// intersperseComments consumes all comments that appear before the next token
// tok and prints it together with the buffered whitespace (i.e., the whitespace
// that needs to be written before the next token). A heuristic is used to mix
// the comments and whitespace. The intersperseComments result indicates if a
// newline was written or if a formfeed was dropped from the whitespace buffer.
func (p *printer) intersperseComments(next token.Position, tok token.Token) (wroteNewline, droppedFF bool) {
	var last *ast.Comment
	for p.commentBefore(next) {
		for _, c := range p.comment.List {
			p.writeCommentPrefix(p.posFor(c.Pos()), next, last, c, tok)
			p.writeComment(c)
			last = c
		}
		p.nextComment()
	}

	if last == nil {
		// no comment was written - we should never reach here since
		// intersperseComments should not be called in that case
		panic("intersperseComments called without pending comments")
	}

	return p.writeCommentSuffix(false)
}

//-------------------------------------------------------------------
// Comments

// setComment sets g as the next comment if g != nil and if node comments
// are enabled - this mode is used when printing source code fragments such
// as exports only. It assumes that there is no pending comment in p.comments
// and at most one pending comment in the p.comment cache.
func (p *printer) setComment(g *ast.CommentGroup) {
	if p.ignoreNodeComments {
		return
	}

	if p.comments == nil {
		// initialize p.comments lazily
		p.comments = make([]*ast.CommentGroup, 1)
	} else if p.cindex < len(p.comments) {
		panic("setComment found pending comments")
	}

	p.comments[0] = g
	p.cindex = 0

	// don't overwrite any pending comment in the p.comment cache
	// (there may be a pending comment when a line comment is
	// immediately followed by a lead comment with no other
	// tokens between)
	if p.coffset == infinity {
		p.nextComment() // get comment ready for use
	}
}

// nextComment stages the next comment for usage.
func (p *printer) nextComment() {
	for p.cindex < len(p.comments) {
		c := p.comments[p.cindex]
		p.cindex++

		if c == nil {
			// This should only happen during initialization when we
			// create an empty comment list. After that this should never
			// happen but handle gracefully anyways.
			continue
		}

		if list := c.List; len(list) > 0 {
			p.comment = c
			p.coffset = p.posFor(list[0].Pos()).Offset
			p.cnewline = p.commentsHaveNewline(list)
			return
		}

		// we should not reach here (correct ASTs don't have empty
		// ast.CommentGroup nodes), but be conservative and try again
	}

	// no more comments
	p.coffset = infinity
}

// commentsHaveNewline reports whether a list of comments belonging to
// an *ast.CommentGroup contains newlines. Because the position information
// may only be partially correct, we also have to read the comment text.
func (p *printer) commentsHaveNewline(list []*ast.Comment) bool {
	// len(list) > 0
	line := p.lineFor(list[0].Pos())
	for i, c := range list {
		if i > 0 && p.lineFor(list[i].Pos()) != line {
			// not all comments on the same line
			return true
		}

		if t := c.Text; len(t) >= 2 && (t[1] == '/' || strings.Contains(t, "\n")) {
			return true
		}
	}

	return false
}

// commentBefore reports whether the current comment group occurs
// before the next position in the source code and printing it does
// not introduce implicit semicolons.
//
func (p *printer) commentBefore(next token.Position) bool {
	return p.coffset < next.Offset && (!p.impliedSemi || !p.cnewline)
}

// commentSizeBefore returns the estimated size of the
// comments on the same line before the next position.
//
func (p *printer) commentSizeBefore(next token.Position) int {
	// save/restore current p.commentInfo (p.nextComment() modifies it)
	defer func(info commentInfo) {
		p.commentInfo = info
	}(p.commentInfo)

	size := 0
	for p.commentBefore(next) {
		for _, c := range p.comment.List {
			size += len(c.Text)
		}
		p.nextComment()
	}
	return size
}

//-------------------------------------------------------------------
// Positioning

// posFor returns the full positional information for a token.Pos.
func (p *printer) posFor(pos token.Pos) token.Position {
	// not used frequently enough to cache entire token.Position
	return p.fset.Position(pos)
}

// lineFor returns the line for a token.Pos.
func (p *printer) lineFor(pos token.Pos) int {
	return p.posFor(pos).Line
}

// linesFrom returns the number of output lines between the current
// output line and the line argument, ignoring any pending (not yet
// emitted) whitespace or comments. It is used to compute an accurate
// size (in number of lines) for a formatted construct.
func (p *printer) linesFrom(line int) int {
	return p.out.Line - line
}

// recordLine records the output line number for the next non-whitespace
// token in *linePtr. It is used to compute an accurate line number for a
// formatted construct, independent of pending (not yet emitted) whitespace
// or comments.
//
func (p *printer) recordLine(linePtr *int) {
	p.linePtr = linePtr
}

//-------------------------------------------------------------------
// Helpers

// Returns true if s contains only white space
// (only tabs and blanks can appear in the printer's context).
//
func isBlank(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > ' ' {
			return false
		}
	}
	return true
}

// commonPrefix returns the common prefix of a and b.
func commonPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] && (a[i] <= ' ' || a[i] == '*') {
		i++
	}
	return a[0:i]
}

// nlines limits n to maxNewlines.
func nlimit(n int) int {
	if n > maxNewlines {
		n = maxNewlines
	}
	return n
}

// trimRight returns s with trailing whitespace removed.
func trimRight(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

// stripCommonPrefix removes a common prefix from /*-style comment lines (unless no
// comment line is indented, all but the first line have some form of space prefix).
// The prefix is computed using heuristics such that is likely that the comment
// contents are nicely laid out after re-printing each line using the printer's
// current indentation.
//
func stripCommonPrefix(lines []string) {
	if len(lines) <= 1 {
		return // at most one line - nothing to do
	}
	// len(lines) > 1

	// The heuristic in this function tries to handle a few
	// common patterns of /*-style comments: Comments where
	// the opening /* and closing */ are aligned and the
	// rest of the comment text is aligned and indented with
	// blanks or tabs, cases with a vertical "line of stars"
	// on the left, and cases where the closing */ is on the
	// same line as the last comment text.

	// Compute maximum common white prefix of all but the first,
	// last, and blank lines, and replace blank lines with empty
	// lines (the first line starts with /* and has no prefix).
	// In cases where only the first and last lines are not blank,
	// such as two-line comments, or comments where all inner lines
	// are blank, consider the last line for the prefix computation
	// since otherwise the prefix would be empty.
	//
	// Note that the first and last line are never empty (they
	// contain the opening /* and closing */ respectively) and
	// thus they can be ignored by the blank line check.
	prefix := ""
	prefixSet := false
	if len(lines) > 2 {
		for i, line := range lines[1 : len(lines)-1] {
			if isBlank(line) {
				lines[1+i] = "" // range starts with lines[1]
			} else {
				if !prefixSet {
					prefix = line
					prefixSet = true
				}
				prefix = commonPrefix(prefix, line)
			}

		}
	}
	// If we don't have a prefix yet, consider the last line.
	if !prefixSet {
		line := lines[len(lines)-1]
		prefix = commonPrefix(line, line)
	}

	/*
	 * Check for vertical "line of stars" and correct prefix accordingly.
	 */
	lineOfStars := false
	if i := strings.Index(prefix, "*"); i >= 0 {
		// Line of stars present.
		if i > 0 && prefix[i-1] == ' ' {
			i-- // remove trailing blank from prefix so stars remain aligned
		}
		prefix = prefix[0:i]
		lineOfStars = true
	} else {
		// No line of stars present.
		// Determine the white space on the first line after the /*
		// and before the beginning of the comment text, assume two
		// blanks instead of the /* unless the first character after
		// the /* is a tab. If the first comment line is empty but
		// for the opening /*, assume up to 3 blanks or a tab. This
		// whitespace may be found as suffix in the common prefix.
		first := lines[0]
		if isBlank(first[2:]) {
			// no comment text on the first line:
			// reduce prefix by up to 3 blanks or a tab
			// if present - this keeps comment text indented
			// relative to the /* and */'s if it was indented
			// in the first place
			i := len(prefix)
			for n := 0; n < 3 && i > 0 && prefix[i-1] == ' '; n++ {
				i--
			}
			if i == len(prefix) && i > 0 && prefix[i-1] == '\t' {
				i--
			}
			prefix = prefix[0:i]
		} else {
			// comment text on the first line
			suffix := make([]byte, len(first))
			n := 2 // start after opening /*
			for n < len(first) && first[n] <= ' ' {
				suffix[n] = first[n]
				n++
			}
			if n > 2 && suffix[2] == '\t' {
				// assume the '\t' compensates for the /*
				suffix = suffix[2:n]
			} else {
				// otherwise assume two blanks
				suffix[0], suffix[1] = ' ', ' '
				suffix = suffix[0:n]
			}
			// Shorten the computed common prefix by the length of
			// suffix, if it is found as suffix of the prefix.
			prefix = strings.TrimSuffix(prefix, string(suffix))
		}
	}

	// Handle last line: If it only contains a closing */, align it
	// with the opening /*, otherwise align the text with the other
	// lines.
	last := lines[len(lines)-1]
	closing := "*/"
	i := strings.Index(last, closing) // i >= 0 (closing is always present)
	if isBlank(last[0:i]) {
		// last line only contains closing */
		if lineOfStars {
			closing = " */" // add blank to align final star
		}
		lines[len(lines)-1] = prefix + closing
	} else {
		// last line contains more comment text - assume
		// it is aligned like the other lines and include
		// in prefix computation
		prefix = commonPrefix(prefix, last)
	}

	// Remove the common prefix from all but the first and empty lines.
	for i, line := range lines {
		if i > 0 && line != "" {
			lines[i] = line[len(prefix):]
		}
	}
}

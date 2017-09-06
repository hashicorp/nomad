// Package eval contains an evaluator for the Sentinel policy language.
package eval

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/lang/token"
	"github.com/hashicorp/sentinel/runtime/encoding"
	"github.com/hashicorp/sentinel/runtime/importer"
	"github.com/hashicorp/sentinel/runtime/trace"
)

type EvalOpts struct {
	Compiled *Compiled         // Compiled policy to evaluate
	Scope    *object.Scope     // Global scope, the parent of this should be Universe
	Importer importer.Importer // Importer for imports
	Timeout  time.Duration     // Max execution time
	Trace    *trace.Trace      // Set to non-nil to recording tracing information
}

// Eval evaluates the compiled policy and returns the result.
//
// err may be an "UndefError" which specifically represents than an "undefined"
// value bubbled all the way to the final result of the policy. This usually
// represents erroneous policy logic.
//
// err may also be "EvalError" which contains more details about the
// environment that resulted in the error.
func Eval(opts *EvalOpts) (bool, error) {
	// Build the evaluation state
	state := &evalState{
		ExecId:   atomic.AddUint64(&globalExecId, 1),
		File:     opts.Compiled.file,
		FileSet:  opts.Compiled.fileSet,
		Scope:    opts.Scope,
		Importer: opts.Importer,
		Timeout:  opts.Timeout,
		Trace:    opts.Trace,
	}

	result, err := state.Eval()

	// If we're tracing, record the final results
	if state.Trace != nil {
		state.Trace.Desc = opts.Compiled.file.Doc.Text()
		state.Trace.Result = result
		state.Trace.Err = err
	}

	return result, err
}

// globalExecId is the global execution ID. If an execution ID pointer
// isn't given in EvalOpts, this ID is incremented and used.
var globalExecId uint64

// Interpreter is the interpreter for Sentinel, implemented using a basic
// AST walk. After calling any methods of Interpreter, the exported fields
// should not be modified.
type evalState struct {
	ExecId   uint64            // Execution ID, unique per evaluation
	File     *ast.File         // File to execute
	FileSet  *token.FileSet    // FileSet for positional information
	Scope    *object.Scope     // Global scope, the parent of this should be Universe
	Importer importer.Importer // Importer for imports
	Timeout  time.Duration     // Timeout for execution
	Trace    *trace.Trace      // If non-nil, sets trace data here

	deadline  time.Time             // deadline of this execution
	imports   map[string]sdk.Import // imports are loaded imports
	returnObj object.Object         // object set by last `return` statement
	timeoutCh <-chan time.Time      // closed after a timeout is reached

	// Trace data

	ruleMap    map[token.Pos]token.Pos   // map from expr position to assignment
	traces     map[token.Pos]*trace.Rule // rule traces
	traceStack []*trace.Bool             // current trace, nil if no trace active
}

// Eval evaluates the policy and returns the resulting value.
//
// err may be an "UndefError" which specifically represents than an "undefined"
// value bubbled all the way to the final result of the policy. This usually
// represents erroneous policy logic.
//
// err may also be "EvalError" which contains more details about the
// environment that resulted in the error.
func (e *evalState) Eval() (result bool, err error) {
	// Setup the timeout channel if we have a timeout
	if e.Timeout > 0 {
		e.deadline = time.Now().UTC().Add(e.Timeout)
		e.timeoutCh = time.After(e.Timeout)
	}

	// If we have tracing enabled, initialize the trace structure
	if e.Trace != nil {
		*e.Trace = trace.Trace{}
		e.ruleMap = make(map[token.Pos]token.Pos)
	}

	// Evaluate. We do this in an inline closure so that we can catch
	// the bailout but continue to setup trace information.
	var obj object.Object
	func() {
		defer e.recoverBailout(&result, &err)
		obj = e.eval(e.File, e.Scope)
	}()

	// If we have tracing enabled, then finalize all the tracing output
	if e.Trace != nil {
		rules := make(map[string]*trace.Rule, len(e.traces))
		for _, t := range e.traces {
			rules[t.Ident] = t
		}

		e.Trace.Rules = rules
	}

	// If an error occurred, return that
	if err != nil {
		return false, err
	}

	// The result can be undefined, which is a special error type
	if undef, ok := obj.(*object.UndefinedObj); ok {
		return false, &UndefError{
			FileSet: e.FileSet,
			Pos:     undef.Pos,
		}
	}

	boolObj, ok := obj.(*object.BoolObj)
	if !ok {
		return false, fmt.Errorf("Top-level result must be boolean, got: %s", obj.Type())
	}

	return boolObj.Value, nil
}

// ----------------------------------------------------------------------------
// Errors

// A bailout panic is raised to indicate early termination.
type bailout struct{ Err error }

// recoverBailout is the deferred function that catches bailout
// errors and sets the proper result and error.
func (e *evalState) recoverBailout(result *bool, err *error) {
	if e := recover(); e != nil {
		// resume same panic if it's not a bailout
		bo, ok := e.(bailout)
		if !ok {
			panic(e)
		}

		// Bailout always results in policy failure
		*result = false
		*err = bo.Err
	}
}

// EvalError is the most common error type returned by Eval.
type EvalError struct {
	Message string
	Scope   *object.Scope
	FileSet *token.FileSet
	Pos     token.Pos
}

func (e *EvalError) Error() string {
	if e.FileSet == nil {
		return fmt.Sprintf("At unknown location: %s", e.Message)
	}

	pos := e.FileSet.Position(e.Pos)
	return fmt.Sprintf("%s: %s", pos, e.Message)
}

// UndefError is returned when the top-level result is undefined.
type UndefError struct {
	FileSet *token.FileSet // FileSet to look up positions
	Pos     []token.Pos    // Positions where undefined was created
}

func (e *UndefError) Error() string {
	if e.FileSet == nil {
		return fmt.Sprintf("undefined behavior at unknown location")
	}

	// Get all the positions
	locs := make([]string, len(e.Pos))
	for i, p := range e.Pos {
		locs[i] = e.FileSet.Position(p).String()
	}

	return fmt.Sprintf(errUndefined, strings.Join(locs, "\n"))
}

// err is a shortcut to easily produce errors from evaluation.
func (e *evalState) err(msg string, n ast.Node, s *object.Scope) {
	panic(bailout{Err: &EvalError{
		Message: msg,
		FileSet: e.FileSet,
		Pos:     n.Pos(),
		Scope:   s,
	}})
}

// TimeoutError is returned when the execution times out.
type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf(errTimeout, e.Timeout)
}

// ----------------------------------------------------------------------------
// Timeouts

func (e *evalState) checkTimeout() {
	select {
	case <-e.timeoutCh:
		// Timeout!
		panic(bailout{Err: &TimeoutError{Timeout: e.Timeout}})

	default:
		// We're okay.
	}
}

// ----------------------------------------------------------------------------
// Evaluation

func (e *evalState) eval(raw ast.Node, s *object.Scope) object.Object {
	// Check the timeout at every point we evaluate. This is a really coarse
	// scope and we can increase performance by making this more fine-grained
	// to nodes that are actually slow.
	e.checkTimeout()

	switch n := raw.(type) {
	case nil:
		// Do nothing
		return nil

	// File
	case *ast.File:
		return e.evalFile(n, s)

	// Statements

	case *ast.AssignStmt:
		e.evalAssign(n, s)

	case *ast.BlockStmt:
		return e.evalBlockStmt(n, s)

	case *ast.IfStmt:
		return e.evalIfStmt(n, s)

	case *ast.ForStmt:
		return e.evalForStmt(n, s)

	case *ast.ReturnStmt:
		e.returnObj = e.eval(n.Result, s)
		return nil

	case *ast.ExprStmt:
		e.eval(n.X, s)
		e.returnObj = nil
		return nil

	// Expressions

	case *ast.Ident:
		if n.Name == UndefinedName {
			return &object.UndefinedObj{Pos: []token.Pos{raw.Pos()}}
		}

		obj := s.Lookup(n.Name)
		if obj == nil {
			switch n.Name {
			case "error":
				return object.ExternalFunc(e.funcError)

			case "print":
				return object.ExternalFunc(e.funcPrint)

			default:
				if _, ok := e.imports[n.Name]; ok {
					e.err(fmt.Sprintf(
						"import %q cannot be accessed without a selector expression",
						n.Name), raw, s)
				}

				e.err(fmt.Sprintf("unknown identifier accessed: %s", n.Name), raw, s)
			}
		}

		if r, ok := obj.(*object.RuleObj); ok {
			return e.evalRuleObj(n.Name, r)
		}

		return obj

	case *ast.BasicLit:
		return e.evalBasicLit(n, s)

	case *ast.ListLit:
		elts := make([]object.Object, len(n.Elts))
		for i, n := range n.Elts {
			elts[i] = e.eval(n, s)
		}

		return &object.ListObj{Elts: elts}

	case *ast.MapLit:
		elts := make([]object.KeyedObj, len(n.Elts))
		for i, n := range n.Elts {
			kv := n.(*ast.KeyValueExpr)
			elts[i] = object.KeyedObj{
				Key:   e.eval(kv.Key, s),
				Value: e.eval(kv.Value, s),
			}
		}

		return &object.MapObj{Elts: elts}

	case *ast.RuleLit:
		// Just set the rule literal, we don't evaluate
		return &object.RuleObj{WhenExpr: n.When, Expr: n.Expr, Scope: s}

	case *ast.FuncLit:
		return &object.FuncObj{Params: n.Params, Body: n.Body.List, Scope: s}

	case *ast.UnaryExpr:
		return e.evalUnaryExpr(n, s)

	case *ast.BinaryExpr:
		return e.evalBinaryExpr(n, s)

	case *ast.IndexExpr:
		return e.evalIndexExpr(n, s)

	case *ast.SliceExpr:
		return e.evalSliceExpr(n, s)

	case *ast.QuantExpr:
		return e.evalQuantExpr(n, s)

	case *ast.CallExpr:
		return e.evalCallExpr(n, s)

	case *ast.SelectorExpr:
		return e.evalSelectorExpr(n, s)

	case *ast.ParenExpr:
		return e.eval(n.X, s)

	// Custom AST nodes

	case *astImportExpr:
		return e.evalImportExpr(n, s)

	default:
		e.err(fmt.Sprintf("unexpected AST type: %T", raw), raw, s)
	}

	return nil
}

func (e *evalState) evalAssign(n *ast.AssignStmt, s *object.Scope) {
	if n.Tok != token.ASSIGN {
		e.err(
			fmt.Sprintf("unsupported assignment token: %s", n.Tok),
			n, s)
	}

	// Determine the LHS
	switch lhs := n.Lhs.(type) {
	case *ast.Ident:
		obj := e.eval(n.Rhs, s)
		dstS := s.Scope(lhs.Name)
		dstS.Objects[lhs.Name] = obj

		// If we're tracing and this is a rule assignment, record the
		// assignment location so that we can use that.
		if e.Trace != nil {
			if r, ok := obj.(*object.RuleObj); ok {
				e.ruleMap[r.Expr.Pos()] = lhs.Pos()
			}
		}

	default:
		e.err(
			fmt.Sprintf("unsupported left-hand side of assignment: %T", n.Lhs),
			n, s)
	}
}

func (e *evalState) evalBlockStmt(n *ast.BlockStmt, s *object.Scope) object.Object {
	for _, stmt := range n.List {
		e.eval(stmt, s)
		if e.returnObj != nil {
			return nil
		}
	}

	return nil
}

func (e *evalState) evalIfStmt(n *ast.IfStmt, s *object.Scope) object.Object {
	// Evaluate condition
	x := e.eval(n.Cond, s)
	b, ok := x.(*object.BoolObj)
	if !ok {
		e.err(
			fmt.Sprintf("if condition must result in a boolean, got %s", x.Type()),
			n.Cond, s)
	}

	if b.Value {
		return e.evalBlockStmt(n.Body, s)
	}

	return e.eval(n.Else, s)
}

func (e *evalState) evalForStmt(n *ast.ForStmt, s *object.Scope) object.Object {
	// Create a new scope
	s = object.NewScope(s)

	// Loop over the elements
	var result object.Object
	e.eltLoop(n.Expr, s, n.Name, n.Name2, func(s *object.Scope) bool {
		result = e.eval(n.Body, s)
		return result == nil
	})

	return result
}

func (e *evalState) evalBasicLit(n *ast.BasicLit, s *object.Scope) object.Object {
	switch n.Kind {
	case token.INT:
		v, err := strconv.ParseInt(n.Value, 0, 64)
		if err != nil {
			e.err(err.Error(), n, s)
		}

		return &object.IntObj{Value: v}

	case token.FLOAT:
		v, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			e.err(err.Error(), n, s)
		}

		return &object.FloatObj{Value: v}

	case token.STRING:
		v, err := strconv.Unquote(n.Value)
		if err != nil {
			e.err(err.Error(), n, s)
		}

		return &object.StringObj{Value: v}

	default:
		e.err(fmt.Sprintf("unknown basic literal type: %s", n.Kind), n, s)
		return nil
	}
}

func (e *evalState) evalFile(n *ast.File, s *object.Scope) object.Object {
	e.imports = make(map[string]sdk.Import)

	// Find implicit imports in our scope
	for k, obj := range s.Objects {
		if r, ok := obj.(*object.RuntimeObj); ok {
			// Remove the runtime object
			delete(s.Objects, k)

			// If it is an import, then we store it directly
			impt, ok := r.Value.(sdk.Import)
			if !ok {
				e.err(fmt.Sprintf(
					"internal error: runtime object %q unknown type %T",
					k, obj), n, s)
			}

			e.imports[k] = impt
		}
	}

	// Load the imports
	for _, impt := range n.Imports {
		// This shouldn't be possible since the parser verifies it.
		if impt.Path.Kind != token.STRING {
			e.err(fmt.Sprintf(
				"internal error, import path is not a string: %s", impt), impt, s)
		}

		path, err := strconv.Unquote(impt.Path.Value)
		if err != nil {
			e.err(err.Error(), impt, s)
		}

		if e.Importer == nil {
			e.err(fmt.Sprintf("import %q not found", path), impt, s)
		}

		obj, err := e.Importer.Import(path)
		if err != nil {
			e.err(err.Error(), impt, s)
		}

		name := path
		if impt.Name != nil {
			name = impt.Name.Name
		}

		e.imports[name] = obj
	}

	// Execute the statements
	for _, stmt := range n.Stmts {
		e.eval(stmt, s)
	}

	// Look up the "main" entrypoint
	obj := s.Lookup("main")
	if obj == nil {
		e.err(errNoMain, n, s)
	}

	// If the top-level object is a rule, then we evaluate it
	if r, ok := obj.(*object.RuleObj); ok {
		obj = e.evalRuleObj("main", r)
	}

	return obj
}

func (e *evalState) evalIndexExpr(n *ast.IndexExpr, s *object.Scope) object.Object {
	// Get the lhs
	x := e.eval(n.X, s)
	if x == object.Null {
		return &object.UndefinedObj{Pos: []token.Pos{n.X.Pos()}}
	}

	// Evaluate the index
	idxRaw := e.eval(n.Index, s)

	switch v := x.(type) {
	case *object.ListObj:
		idx, ok := idxRaw.(*object.IntObj)
		if !ok {
			e.err(fmt.Sprintf("indexing a list requires an int key, got %s", idxRaw.Type()), n, s)
		}

		idxVal := idx.Value
		if idxVal >= int64(len(v.Elts)) {
			return &object.UndefinedObj{Pos: []token.Pos{n.X.Pos()}}
		}

		if idxVal < 0 {
			idxVal += int64(len(v.Elts))
		}

		return v.Elts[idxVal]

	case *object.MapObj:
		for _, elt := range v.Elts {
			// This is terrible.
			if elt.Key.Type() == idxRaw.Type() && elt.Key.String() == idxRaw.String() {
				return elt.Value
			}
		}

		return &object.UndefinedObj{Pos: []token.Pos{n.X.Pos()}}

	default:
		e.err(fmt.Sprintf("only a list or map can be indexed, got %s", x.Type()), n, s)
		return nil
	}
}

func (e *evalState) evalSliceExpr(n *ast.SliceExpr, s *object.Scope) object.Object {
	// Get the lhs
	xRaw := e.eval(n.X, s)
	x, ok := xRaw.(*object.ListObj)
	if !ok {
		e.err(fmt.Sprintf("only a list can be sliced, got %s", xRaw.Type()), n, s)
	}

	// Setup the low/high for slicing
	var low, high int

	// Get the low value if it is set
	if n.Low != nil {
		raw := e.eval(n.Low, s)
		v, ok := raw.(*object.IntObj)
		if !ok {
			e.err(fmt.Sprintf("slice index must be an int, got %s", raw.Type()), n.Low, s)
		}

		low = int(v.Value)
	}

	// Get the high value if it is set
	if n.High != nil {
		raw := e.eval(n.High, s)
		v, ok := raw.(*object.IntObj)
		if !ok {
			e.err(fmt.Sprintf("slice index must be an int, got %s", raw.Type()), n.Low, s)
		}

		high = int(v.Value)
	} else {
		high = len(x.Elts)
	}

	return &object.ListObj{Elts: x.Elts[low:high]}
}

func (e *evalState) evalQuantExpr(n *ast.QuantExpr, s *object.Scope) object.Object {
	// Create a new scope
	s = object.NewScope(s)

	// TODO: undefined

	// Loop over the elements
	var result object.Object
	e.eltLoop(n.Expr, s, n.Name, n.Name2, func(s *object.Scope) bool {
		// Evaluate the body
		x := e.eval(n.Body, s)

		b, ok := x.(*object.BoolObj)
		if !ok {
			e.err(fmt.Sprintf("body of quantifier expression must result in bool"), n.Body, s)
		}

		switch {
		case n.Op == token.ANY && b.Value:
			result = object.True
			return false

		case n.Op == token.ALL && !b.Value:
			result = object.False
			return false
		}

		return true
	})

	if result == nil {
		result = object.Bool(n.Op == token.ALL)
	}

	return result
}

func (e *evalState) eltLoop(n ast.Expr, s *object.Scope, n1, n2 *ast.Ident, f func(*object.Scope) bool) {
	raw := e.eval(n, s)
	switch x := raw.(type) {
	case *object.ListObj:
		idx := n1
		value := n2
		if n2 == nil {
			idx = nil
			value = n1
		}

		for i, elt := range x.Elts {
			if idx != nil {
				s.Objects[idx.Name] = &object.IntObj{Value: int64(i)}
			}
			if value != nil {
				s.Objects[value.Name] = elt
			}

			cont := f(s)
			if !cont {
				break
			}
		}

	default:
		e.err(fmt.Sprintf("unsupported type for looping: %s", raw.Type()), n, s)
	}
}

func (e *evalState) evalUnaryExpr(n *ast.UnaryExpr, s *object.Scope) object.Object {
	switch n.Op {
	case token.NOT, token.NOTSTR:
		// Evaluate the field
		x := e.eval(n.X, s)

		// If it is undefined, it is undefined
		if _, ok := x.(*object.UndefinedObj); ok {
			return x
		}

		bx, ok := x.(*object.BoolObj)
		if !ok {
			e.err(fmt.Sprintf("operand needs to be boolean, got %s", x.Type()), n, s)
		}

		// Invert and return
		return object.Bool(!bx.Value)

	case token.SUB:
		// Evaluate the field
		x := e.eval(n.X, s)

		// If it is undefined, it is undefined
		if _, ok := x.(*object.UndefinedObj); ok {
			return x
		}

		// It must be an int
		switch v := x.(type) {
		case *object.UndefinedObj:
			return v

		case *object.IntObj:
			return &object.IntObj{Value: -v.Value}

		case *object.FloatObj:
			return &object.FloatObj{Value: -v.Value}

		default:
			e.err(fmt.Sprintf("unary negation requires int or float, got %s", v.Type()), n, s)
			return nil
		}
	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}
}

func (e *evalState) evalBinaryExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	switch n.Op {
	case token.LAND, token.LOR, token.LXOR:
		return e.evalBooleanExpr(n, s)

	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ,
		token.IS, token.ISNOT:
		return e.evalRelExpr(n, s)

	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		return e.evalMathExpr(n, s)

	case token.CONTAINS:
		return e.evalBinaryNeg(n, e.evalContainsExpr(n, s))

	case token.IN:
		return e.evalBinaryNeg(n, e.evalInExpr(n, s))

	case token.MATCHES:
		return e.evalBinaryNeg(n, e.evalMatchesExpr(n, s))

	case token.ELSE:
		return e.evalElseExpr(n, s)

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}
}

func (e *evalState) evalBinaryNeg(n *ast.BinaryExpr, obj object.Object) object.Object {
	if !n.OpNeg {
		return obj
	}

	switch x := obj.(type) {
	case *object.BoolObj:
		return object.Bool(!x.Value)

	default:
		// Invalid types are handled elsewhere
		return obj
	}
}

func (e *evalState) evalBooleanExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Eval left operand
	e.pushTrace(n.X)
	x := e.popTrace(e.eval(n.X, s))

	// If x is undefined, switch to undefined logic
	if _, ok := x.(*object.UndefinedObj); ok {
		// If we're in an AND, this is failing no matter what so we don't
		// evaluate y.
		var y object.Object
		if n.Op != token.LAND {
			e.pushTrace(n.Y)
			y = e.popTrace(e.eval(n.Y, s))
		}

		return e.evalBooleanExpr_undef(n, s, x, y)
	}

	// Verify X is a boolean
	bx, ok := x.(*object.BoolObj)
	if !ok {
		e.err(fmt.Sprintf("left operand needs to be boolean, got %s", x.Type()), n, s)
	}

	// Short circuit-logic
	if n.Op == token.LAND && !bx.Value {
		return object.False
	} else if n.Op == token.LOR && bx.Value {
		return object.True
	}

	// Eval right operand
	e.pushTrace(n.Y)
	y := e.popTrace(e.eval(n.Y, s))

	// If y is undefined it is always undefined
	if _, ok := y.(*object.UndefinedObj); ok {
		return e.evalBooleanExpr_undef(n, s, y, x)
	}

	by, ok := y.(*object.BoolObj)
	if !ok {
		e.err(fmt.Sprintf("right operand needs to be boolean, got %s", y.Type()), n, s)
	}

	// Perform the actual logic
	switch n.Op {
	case token.LAND:
		return object.Bool(bx.Value && by.Value)

	case token.LOR:
		return object.Bool(bx.Value || by.Value)

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}
}

func (e *evalState) evalBooleanExpr_undef(
	n *ast.BinaryExpr, s *object.Scope,
	undef object.Object, y object.Object) object.Object {
	// If it is not OR, its always going to be undefined
	if n.Op != token.LOR {
		return undef
	}

	// If y is true then we escape
	if b, ok := y.(*object.BoolObj); ok && b.Value {
		return b
	}

	// Otherwise, undefined
	return undef
}

func (e *evalState) evalMathExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Eval left operand
	x := e.eval(n.X, s)
	if _, ok := x.(*object.UndefinedObj); ok {
		return x
	}

	// Eval right operand
	y := e.eval(n.Y, s)
	if _, ok := y.(*object.UndefinedObj); ok {
		return y
	}

	// They have to be the same type to compare. This is pretty slow way to
	// check this but works for now.
	xtyp := reflect.TypeOf(x)
	ytyp := reflect.TypeOf(y)
	if xtyp != ytyp {
		// We allow float/int comparisons as a specific exception to the types
		switch {
		// TODO: float/int

		default:
			e.err(fmt.Sprintf(
				"comparison requires both operands to be the same type, got %s and %s",
				x.Type(), y.Type()),
				n, s)
		}
	}

	switch xObj := x.(type) {
	case *object.IntObj:
		return e.evalMathExpr_int(n, s, xObj.Value, y.(*object.IntObj).Value)

	case *object.ListObj:
		return e.evalMathExpr_list(n, s, xObj, y.(*object.ListObj))

	// TODO: floats
	// TODO: strings

	default:
		e.err(fmt.Sprintf("can't perform math on type %s", x.Type()), n, s)
		return nil
	}
}

func (e *evalState) evalMathExpr_int(n *ast.BinaryExpr, s *object.Scope, x, y int64) object.Object {
	var result int64
	switch n.Op {
	case token.ADD:
		result = x + y

	case token.SUB:
		result = x - y

	case token.MUL:
		result = x * y

	case token.QUO:
		result = x / y

	case token.REM:
		result = x % y

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}

	return &object.IntObj{Value: result}
}

func (e *evalState) evalMathExpr_list(n *ast.BinaryExpr, s *object.Scope, x, y *object.ListObj) object.Object {
	switch n.Op {
	case token.ADD:
		xLen := len(x.Elts)
		elts := make([]object.Object, xLen+len(y.Elts))
		copy(elts, x.Elts)
		copy(elts[xLen:], y.Elts)
		return &object.ListObj{Elts: elts}

	default:
		e.err(fmt.Sprintf("unsupported operator for lists: %s", n.Op), n, s)
		return nil
	}
}

func (e *evalState) evalRelExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Eval left and right operand
	x := e.eval(n.X, s)
	y := e.eval(n.Y, s)

	// Undefined short-circuiting
	if _, ok := x.(*object.UndefinedObj); ok {
		return x
	}
	if _, ok := y.(*object.UndefinedObj); ok {
		return y
	}

	// Special-case null comparison
	if x == object.Null || y == object.Null {
		switch n.Op {
		case token.EQL, token.IS:
			return object.Bool(x == y)

		case token.NEQ, token.ISNOT:
			return object.Bool(x != y)

		default:
			e.err(fmt.Sprintf("null may only be used with equality checks, got: %s", n.Op), n, s)
			return nil
		}
	}

	// They have to be the same type to compare. This is pretty slow way to
	// check this but works for now.
	xtyp := x.Type()
	ytyp := y.Type()
	if xtyp != ytyp {
		// We allow float/int comparisons as a specific exception to the types
		switch {
		case xtyp == object.INT && ytyp == object.FLOAT:
			x = &object.FloatObj{Value: float64(x.(*object.IntObj).Value)}

		case ytyp == object.INT && xtyp == object.FLOAT:
			y = &object.FloatObj{Value: float64(y.(*object.IntObj).Value)}

		default:
			e.err(fmt.Sprintf(
				"comparison requires both operands to be the same type, got %s and %s",
				x.Type(), y.Type()),
				n, s)
		}
	}

	switch xObj := x.(type) {
	case *object.BoolObj:
		return e.evalRelExpr_bool(n, s, xObj.Value, y.(*object.BoolObj).Value)

	case *object.IntObj:
		return e.evalRelExpr_int(n, s, xObj.Value, y.(*object.IntObj).Value)

	case *object.FloatObj:
		return e.evalRelExpr_float(n, s, xObj.Value, y.(*object.FloatObj).Value)

	case *object.StringObj:
		return e.evalRelExpr_string(n, s, xObj.Value, y.(*object.StringObj).Value)

	default:
		e.err(fmt.Sprintf("can't compare type %s", x.Type()), n, s)
		return nil
	}
}

func (e *evalState) evalRelExpr_bool(n *ast.BinaryExpr, s *object.Scope, x, y bool) object.Object {
	var result bool
	switch n.Op {
	case token.EQL, token.IS:
		result = x == y

	case token.NEQ, token.ISNOT:
		result = x != y

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}

	return object.Bool(result)
}

func (e *evalState) evalRelExpr_int(n *ast.BinaryExpr, s *object.Scope, x, y int64) object.Object {
	var result bool
	switch n.Op {
	case token.EQL, token.IS:
		result = x == y

	case token.NEQ, token.ISNOT:
		result = x != y

	case token.LSS:
		result = x < y

	case token.GTR:
		result = x > y

	case token.LEQ:
		result = x <= y

	case token.GEQ:
		result = x >= y

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}

	return object.Bool(result)
}

func (e *evalState) evalRelExpr_float(n *ast.BinaryExpr, s *object.Scope, x, y float64) object.Object {
	var result bool
	switch n.Op {
	case token.EQL, token.IS:
		result = x == y

	case token.NEQ, token.ISNOT:
		result = x != y

	case token.LSS:
		result = x < y

	case token.GTR:
		result = x > y

	case token.LEQ:
		result = x <= y

	case token.GEQ:
		result = x >= y

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}

	return object.Bool(result)
}

func (e *evalState) evalRelExpr_string(n *ast.BinaryExpr, s *object.Scope, x, y string) object.Object {
	var result bool
	switch n.Op {
	case token.EQL, token.IS:
		result = x == y

	case token.NEQ, token.ISNOT:
		result = x != y

	case token.LSS:
		result = x < y

	case token.GTR:
		result = x > y

	case token.LEQ:
		result = x <= y

	case token.GEQ:
		result = x >= y

	default:
		e.err(fmt.Sprintf("unsupported operator: %s", n.Op), n, s)
		return nil
	}

	return object.Bool(result)
}

func (e *evalState) evalContainsExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Evaluate the left side
	x := e.eval(n.X, s)

	switch xv := x.(type) {
	case *object.ListObj:
		y := e.eval(n.Y, s)
		return e.evalContainsExpr_list(n, s, xv, y)

	case *object.MapObj:
		y := e.eval(n.Y, s)
		return e.evalContainsExpr_map(n, s, xv, y)

	default:
		e.err(fmt.Sprintf("left operand for contains must be list or map, got %s", x.Type()), n, s)
		return nil
	}
}

func (e *evalState) evalContainsExpr_list(
	n *ast.BinaryExpr, s *object.Scope, x *object.ListObj, y object.Object) object.Object {
	ytyp := reflect.TypeOf(y)
	for _, elt := range x.Elts {
		if reflect.TypeOf(elt) == ytyp && reflect.DeepEqual(elt, y) {
			return object.True
		}
	}

	return object.False
}

func (e *evalState) evalContainsExpr_map(
	n *ast.BinaryExpr, s *object.Scope, x *object.MapObj, y object.Object) object.Object {
	ytyp := reflect.TypeOf(y)
	for _, kv := range x.Elts {
		elt := kv.Key
		if reflect.TypeOf(elt) == ytyp && reflect.DeepEqual(elt, y) {
			return object.True
		}
	}

	return object.False
}

func (e *evalState) evalInExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Evaluate the left side
	x := e.eval(n.X, s)
	if x.Type() == object.UNDEFINED {
		return x
	}

	// Evaluate the right side
	y := e.eval(n.Y, s)
	if y.Type() == object.UNDEFINED {
		return y
	}

	switch yv := y.(type) {
	case *object.ListObj:
		return e.evalContainsExpr_list(n, s, yv, x)

	case *object.MapObj:
		return e.evalContainsExpr_map(n, s, yv, x)

	default:
		e.err(fmt.Sprintf("right operand for in must be list or map, got %s", y.Type()), n.Y, s)
		return nil
	}
}

func (e *evalState) evalMatchesExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Evaluate the left side
	raw := e.eval(n.X, s)
	x, ok := raw.(*object.StringObj)
	if !ok {
		// If it is undefined, return it
		if _, ok := raw.(*object.UndefinedObj); ok {
			return raw
		}

		e.err(fmt.Sprintf("left operand for matches must be a string, got %s", raw.Type()), n, s)
	}

	// Evaluate the right
	raw = e.eval(n.Y, s)
	y, ok := raw.(*object.StringObj)
	if !ok {
		// If it is undefined, return it
		if _, ok := raw.(*object.UndefinedObj); ok {
			return raw
		}

		e.err(fmt.Sprintf("right operand for matches must be a string, got %s", raw.Type()), n, s)
	}

	// Parse the regular expression
	re, err := regexp.Compile(y.Value)
	if err != nil {
		e.err(fmt.Sprintf("invalid regular expression: %s", err), n.Y, s)
	}

	return object.Bool(re.MatchString(x.Value))
}

func (e *evalState) evalElseExpr(n *ast.BinaryExpr, s *object.Scope) object.Object {
	// Evaluate the left side
	raw := e.eval(n.X, s)
	if _, ok := raw.(*object.UndefinedObj); !ok {
		// If it isn't undefined, return it
		return raw
	}

	// Evaluate and return the right
	return e.eval(n.Y, s)
}

func (e *evalState) evalCallExpr(n *ast.CallExpr, s *object.Scope) object.Object {
	// Get the function
	raw := e.eval(n.Fun, s)
	switch raw.(type) {
	case *object.FuncObj:
	case *object.ExternalObj:
	default:
		e.err(fmt.Sprintf("attempting to call non-function: %s", raw.Type()), n.Fun, s)
	}

	f, ok := raw.(*object.FuncObj)
	if !ok {
		// It has to be an external from above check
		ext, ok := raw.(*object.ExternalObj)
		if !ok {
			e.err(fmt.Sprintf("attempting to call non-function: %s", raw.Type()), n.Fun, s)
		}

		// Build the args
		args := make([]object.Object, len(n.Args))
		for i, arg := range n.Args {
			args[i] = e.eval(arg, s)
		}

		// Call it
		result, err := ext.External.Call(args)
		if err != nil {
			e.err(err.Error(), n.Fun, s)
		}

		resultObj, err := encoding.GoToObject(result)
		if err != nil {
			e.err(err.Error(), n.Fun, s)
		}

		if u, ok := resultObj.(*object.UndefinedObj); ok && u.Pos == nil {
			u.Pos = []token.Pos{n.Fun.Pos()}
		}

		return resultObj
	}

	// Check argument length
	if f != nil && len(f.Params) != len(n.Args) {
		e.err(fmt.Sprintf(
			"invalid number of arguments, expected %d, got %d",
			len(f.Params), len(n.Args)), n.Fun, s)
	}

	// Create a new scope for the function
	evalScope := object.NewScope(f.Scope)

	// Evaluate and set all the arguments
	for i, arg := range n.Args {
		evalScope.Objects[f.Params[i].Name] = e.eval(arg, s)
	}

	// Evaluate all the statements
	for _, stmt := range f.Body {
		e.eval(stmt, evalScope)
		if e.returnObj != nil {
			result := e.returnObj
			e.returnObj = nil
			return result
		}
	}

	// This shouldn't happen because semantic checks should catch this.
	e.err(errNoReturn, n, s)
	return nil
}

func (e *evalState) evalSelectorExpr(n *ast.SelectorExpr, s *object.Scope) object.Object {
	// Get the value which should be a map
	raw := e.eval(n.X, s)
	switch x := raw.(type) {
	case *object.ExternalObj:
		result, err := x.External.Get(n.Sel.Name)
		if err != nil {
			e.err(err.Error(), n.Sel, s)
		}
		if result == nil {
			return &object.UndefinedObj{Pos: []token.Pos{n.Sel.Pos()}}
		}

		obj, err := encoding.GoToObject(result)
		if err != nil {
			e.err(err.Error(), n.Sel, s)
		}

		return obj
	case *object.MapObj:
		for _, elt := range x.Elts {
			if s, ok := elt.Key.(*object.StringObj); ok && s.Value == n.Sel.Name {
				return elt.Value
			}
		}

		return &object.UndefinedObj{Pos: []token.Pos{n.Sel.Pos()}}

	case *object.UndefinedObj:
		return x

	default:
		e.err(fmt.Sprintf(
			"selectors only available for imports and maps, got %s",
			raw.Type()), n, s)
		return nil
	}
}

func (e *evalState) evalImportExpr(n *astImportExpr, s *object.Scope) object.Object {
	// Lookup the import. If it is shadowed, then fall back.
	if v := s.Lookup(n.Import); v != nil {
		return e.eval(n.Original, s)
	}

	// Not an import, find the import in our map and call it
	impt, ok := e.imports[n.Import]
	if !ok {
		e.err(fmt.Sprintf("import not found: %s", n.Import), n, s)
	}

	// Build the args. This must be nil (not just an empty slice) if
	// we aren't making a call (n.Args == nil).
	var args []interface{}
	if n.Args != nil {
		args = make([]interface{}, len(n.Args))
		for i, arg := range n.Args {
			value, err := encoding.ObjectToGo(e.eval(arg, s), nil)
			if err != nil {
				e.err(fmt.Sprintf("error converting argument: %s", err), n, s)
			}

			args[i] = value
		}
	}

	// Perform the external call
	results, err := impt.Get([]*sdk.GetReq{
		&sdk.GetReq{
			ExecId:       e.ExecId,
			ExecDeadline: e.deadline,
			KeyId:        42,
			Keys:         n.Keys,
			Args:         args,
		},
	})
	if err != nil {
		e.err(err.Error(), n, s)
	}

	// Find our resulting value
	var result *sdk.GetResult
	for _, v := range results {
		if v.KeyId == 42 {
			result = v
			break
		}
	}
	if result == nil || result.Value == sdk.Undefined {
		return &object.UndefinedObj{Pos: []token.Pos{n.Pos()}}
	}

	obj, err := encoding.GoToObject(result.Value)
	if err != nil {
		e.err(err.Error(), n, s)
	}

	return obj
}

func (e *evalState) evalRuleObj(ident string, r *object.RuleObj) object.Object {
	if r.Value != nil {
		return r.Value
	}

	// If this rule has a when predicate, check that.
	if r.WhenExpr != nil {
		// If we haven't evalutated the when expression before, do it
		if r.WhenValue == nil {
			r.WhenValue = e.eval(r.WhenExpr, r.Scope)
		}

		switch v := r.WhenValue.(type) {
		case *object.UndefinedObj:
			// If the predicate is undefined, we continue to chain it
			return r.WhenValue

		case *object.BoolObj:
			// If the predicate failed, then the result of the rule is always true
			if !v.Value {
				r.Value = object.True // Set the memoized value so we don't exec again
				return r.Value
			}

		default:
			e.err("rule predicate evaluated to a non-boolean value", r.WhenExpr, r.Scope)
		}
	}

	// If tracing is enabled and we haven't traced the execution
	// of this rule yet, then we trace the execution of this rule.
	if e.Trace != nil {
		if _, ok := e.traces[r.Expr.Pos()]; !ok {
			if e.traces == nil {
				e.traces = make(map[token.Pos]*trace.Rule)
			}

			pos := r.Expr.Pos()
			if identPos, ok := e.ruleMap[pos]; ok {
				pos = identPos
			}

			// Store the trace
			e.traces[r.Expr.Pos()] = &trace.Rule{
				Ident: ident,
				Pos:   pos,
				Root:  e.pushTrace(r.Expr),
			}
		}
	}

	raw := e.eval(r.Expr, r.Scope)

	// If we're tracing, then we need to restore the old trace.
	if e.Trace != nil {
		e.popTrace(raw)
	}

	// If the rule didn't result in a bool, it is an error
	if raw.Type() != object.BOOL && raw.Type() != object.UNDEFINED {
		e.err("rule evaluated to a non-boolean value", r.Expr, r.Scope)
	}

	r.Value = raw
	return raw
}

// ----------------------------------------------------------------------------
// Error messages

const errNoMain = `No 'main' assignment found.

You must assign a rule to the 'main' variable. This is the entrypoint for
a Sentinel policy. The result of this rule determines the result of the
entire policy.`

const errNoReturn = `No return statement found in the function!

A return statement is required to return a result from a function.`

const errTimeout = `Execution of policy timed out after %s.

Policy execution is limited to %s by the host system. Please modify the
policy to execute within this time.
`

const errUndefined = `Result value was 'undefined'.

This means that undefined behavior was experienced at one or more locations
and the result of your policy ultimately resulted in "undefined". This usual
represents a policy which doesn't handle all possible cases and should be
corrected.

The locations where the undefined behavior happened are:

%s`

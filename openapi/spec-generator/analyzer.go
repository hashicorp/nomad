package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
	"strings"
)

type HTTPProfile struct {
	IsResponseWriter bool // net/http.ResponseWriter
	IsRequest        bool // *net/http.Request
	IsHandler        bool // net/http.Handler
}

// Analyzer provides a number of static analysis helper functions.
type Analyzer struct {
	typesInfos []*types.Info
	TypesInfo  *types.Info
}

// GetSource returns the source code for an ast.Node
func (a *Analyzer) GetSource(elem interface{}, fileSet *token.FileSet) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, elem); err != nil {
		return "", err
	} else {
		return buf.String(), nil
	}
}

func (a *Analyzer) IsHttpHandler(typeDefFunc *types.Func) bool {
	if funcSignature, ok := typeDefFunc.Type().(*types.Signature); ok {
		// typeDefFunc.
		profile := HTTPProfile{}

		a.setHTTPProfile(funcSignature.Params(), &profile)
		a.setHTTPProfile(funcSignature.Results(), &profile)

		return profile.IsHandler || (profile.IsResponseWriter && profile.IsRequest)
	}

	return false
}

func (a *Analyzer) setHTTPProfile(tup *types.Tuple, result *HTTPProfile) {
	if tup == nil {
		return
	}

	for i := 0; i < tup.Len(); i++ {
		tupleMember := tup.At(i)
		objectType := tupleMember.Type().String()
		switch objectType {
		case "net/http.ResponseWriter":
			result.IsResponseWriter = true
		case "*net/http.Request":
			result.IsRequest = true
		case "net/http.Handler":
			result.IsHandler = true
		default:
			// capture cases such as function that return or accept functions
			// ex. (func(net/http.Handler) net/http.Handler, error)
			if strings.Contains(objectType, "net/http.ResponseWriter") {
				result.IsResponseWriter = true
			}
			if strings.Contains(objectType, "*net/http.Request") {
				result.IsRequest = true
			}
			if strings.Contains(objectType, "net/http.Handler") {
				result.IsHandler = true
			}
		}
	}

	return
}

func (a *Analyzer) GetHttpHandlers(pkg *packages.Package) map[string]*types.Func {
	httpHandlers := make(map[string]*types.Func)
	for _, typeDef := range pkg.TypesInfo.Defs {
		if typeDef != nil {
			if typeDefFunc, ok := typeDef.(*types.Func); ok {
				if a.IsHttpHandler(typeDefFunc) {
					httpHandlers[fmt.Sprintf("%s.%s", pkg.Name, typeDefFunc.Name())] = typeDefFunc
				}
			}
		}
	}

	return httpHandlers
}

func (a *Analyzer) GetStructs(pkg *packages.Package) (map[string]*ast.TypeSpec, error) {
	var structMap = make(map[string]*ast.TypeSpec)
	fmtString := pkg.Name + ".%s"

	visitFunc := func(node ast.Node) bool {
		switch node.(type) {
		case *ast.TypeSpec:
			typeSpec := node.(*ast.TypeSpec)
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				structMap[fmt.Sprintf(fmtString, typeSpec.Name.Name)] = typeSpec
			}
		}
		return true
	}

	for _, goFile := range pkg.GoFiles {
		fileSet := token.NewFileSet() // positions are relative to fset
		file, err := parser.ParseFile(fileSet, goFile, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("Analyzer.GetStructs.parser.ParseFile: %v\n", err)
		}

		ast.Inspect(file, visitFunc)
	}

	return structMap, nil
}

func NewTypeProvider(analyzer *Analyzer) *TypeProvider {
	return &TypeProvider{
		analyzer: analyzer,
		Result:   make(map[Constraint]bool),
	}
}

// TypeProvider exists to handle the complexity of resolving the type of ast
// expressions which are hyper-dynamic. Adapted from https://cs.opensource.google/go/x/tools/+/master:refactor/satisfy/find.go
type TypeProvider struct {
	Result         map[Constraint]bool
	methodSetCache typeutil.MethodSetCache

	analyzer *Analyzer
	sig      *types.Signature
}

var (
	tInvalid     = types.Typ[types.Invalid]
	tUntypedBool = types.Typ[types.UntypedBool]
	tUntypedNil  = types.Typ[types.UntypedNil]
)

type Constraint struct {
	LHS, RHS types.Type
}

// GetExprType returns the types.Type of an ast.Expr. Useful for determining
// the type of inputs, outputs, and variables.
func (p *TypeProvider) GetExprType(e ast.Expr) types.Type {
	if p.Result == nil {
		panic("TypeProvider.Result cannot be nil - use NewTypeProvider factory method to construct a valid instance")
	}

	if p.analyzer.TypesInfo == nil {
		panic("TypeProvider.analyzer.TypesInfo cannot be nil - use NewTypeProvider factory method to construct a valid instance")
	}

	tv := p.analyzer.TypesInfo.Types[e]
	if tv.Value != nil {
		return tv.Type // prune the descent for constants
	}

	// tv.Type may be nil for an ast.Ident.

	switch e := e.(type) {
	case *ast.BadExpr, *ast.BasicLit:
		// no-op

	case *ast.Ident:
		// (referring idents only)
		if obj, ok := p.analyzer.TypesInfo.Uses[e]; ok {
			return obj.Type()
		}
		if e.Name == "_" { // e.g. "for _ = range x"
			return tInvalid
		}
		panic("undefined ident: " + e.Name)

	case *ast.Ellipsis:
		if e.Elt != nil {
			p.GetExprType(e.Elt)
		}

	case *ast.FuncLit:
		saved := p.sig
		p.sig = tv.Type.(*types.Signature)
		p.stmt(e.Body)
		p.sig = saved

	case *ast.CompositeLit:
		switch T := p.deref(tv.Type).Underlying().(type) {
		case *types.Struct:
			for i, elem := range e.Elts {
				if kv, ok := elem.(*ast.KeyValueExpr); ok {
					p.assign(p.analyzer.TypesInfo.Uses[kv.Key.(*ast.Ident)].Type(), p.GetExprType(kv.Value))
				} else {
					p.assign(T.Field(i).Type(), p.GetExprType(elem))
				}
			}

		case *types.Map:
			for _, elem := range e.Elts {
				elem := elem.(*ast.KeyValueExpr)
				p.assign(T.Key(), p.GetExprType(elem.Key))
				p.assign(T.Elem(), p.GetExprType(elem.Value))
			}

		case *types.Array, *types.Slice:
			tElem := T.(interface {
				Elem() types.Type
			}).Elem()
			for _, elem := range e.Elts {
				if kv, ok := elem.(*ast.KeyValueExpr); ok {
					// ignore the key
					p.assign(tElem, p.GetExprType(kv.Value))
				} else {
					p.assign(tElem, p.GetExprType(elem))
				}
			}

		default:
			panic("unexpected composite literal type: " + tv.Type.String())
		}

	case *ast.ParenExpr:
		p.GetExprType(e.X)

	case *ast.SelectorExpr:
		if _, ok := p.analyzer.TypesInfo.Selections[e]; ok {
			p.GetExprType(e.X) // selection
		} else {
			return p.analyzer.TypesInfo.Uses[e.Sel].Type() // qualified identifier
		}

	case *ast.IndexExpr:
		x := p.GetExprType(e.X)
		i := p.GetExprType(e.Index)
		if ux, ok := x.Underlying().(*types.Map); ok {
			p.assign(ux.Key(), i)
		}

	case *ast.SliceExpr:
		p.GetExprType(e.X)
		if e.Low != nil {
			p.GetExprType(e.Low)
		}
		if e.High != nil {
			p.GetExprType(e.High)
		}
		if e.Max != nil {
			p.GetExprType(e.Max)
		}

	case *ast.TypeAssertExpr:
		x := p.GetExprType(e.X)
		p.typeAssert(x, p.analyzer.TypesInfo.Types[e.Type].Type)

	case *ast.CallExpr:
		if tvFun := p.analyzer.TypesInfo.Types[e.Fun]; tvFun.IsType() {
			// conversion
			arg0 := p.GetExprType(e.Args[0])
			p.assign(tvFun.Type, arg0)
		} else {
			// function call
			if id, ok := astutil.Unparen(e.Fun).(*ast.Ident); ok {
				if obj, ok := p.analyzer.TypesInfo.Uses[id].(*types.Builtin); ok {
					sig := p.analyzer.TypesInfo.Types[id].Type.(*types.Signature)
					return p.builtin(obj, sig, e.Args, tv.Type)
				}
			}
			// ordinary call
			p.call(p.GetExprType(e.Fun).Underlying().(*types.Signature), e.Args)
		}

	case *ast.StarExpr:
		p.GetExprType(e.X)

	case *ast.UnaryExpr:
		p.GetExprType(e.X)

	case *ast.BinaryExpr:
		x := p.GetExprType(e.X)
		y := p.GetExprType(e.Y)
		if e.Op == token.EQL || e.Op == token.NEQ {
			p.compare(x, y)
		}

	case *ast.KeyValueExpr:
		p.GetExprType(e.Key)
		p.GetExprType(e.Value)

	case *ast.ArrayType,
		*ast.StructType,
		*ast.FuncType,
		*ast.InterfaceType,
		*ast.MapType,
		*ast.ChanType:
		panic(e)
	}

	if tv.Type == nil {
		panic(fmt.Sprintf("no type for %T", e))
	}

	return tv.Type
}

// typeAssert must be called for each type assertion x.(T) where x has
// interface type I.
func (p *TypeProvider) typeAssert(I, T types.Type) {
	// Type assertions are slightly subtle, because they are allowed
	// to be "impossible", e.g.
	//
	// 	var x interface{f()}
	//	_ = x.(interface{f()int}) // legal
	//
	// (In hindsight, the language spec should probably not have
	// allowed this, but it's too late to fix now.)
	//
	// This means that a type assert from I to T isn't exactly a
	// constraint that T is assignable to I, but for a refactoring
	// tool it is a conditional constraint that, if T is assignable
	// to I before a refactoring, it should remain so after.

	if types.AssignableTo(T, I) {
		p.assign(I, T)
	}
}

// deref returns a pointer's element type; otherwise it returns typ.
func (p *TypeProvider) deref(typ types.Type) types.Type {
	if p, ok := typ.Underlying().(*types.Pointer); ok {
		return p.Elem()
	}
	return typ
}

// assign records pairs of distinct types that are related by
// assignability, where the left-hand side is an interface and both
// sides have methods.
//
// It should be called for all assignability checks, type assertions,
// explicit conversions and comparisons between two types, unless the
// types are uninteresting (e.g. lhs is a concrete type, or the empty
// interface; rhs has no methods).
//
func (p *TypeProvider) assign(lhs, rhs types.Type) {
	if types.Identical(lhs, rhs) {
		return
	}
	if !types.IsInterface(lhs) {
		return
	}

	if p.methodSetCache.MethodSet(lhs).Len() == 0 {
		return
	}
	if p.methodSetCache.MethodSet(rhs).Len() == 0 {
		return
	}
	// record the pair
	p.Result[Constraint{lhs, rhs}] = true
}

func (p *TypeProvider) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.BadStmt,
		*ast.EmptyStmt,
		*ast.BranchStmt:
		// no-op

	case *ast.DeclStmt:
		d := s.Decl.(*ast.GenDecl)
		if d.Tok == token.VAR { // ignore consts
			for _, spec := range d.Specs {
				p.valueSpec(spec.(*ast.ValueSpec))
			}
		}

	case *ast.LabeledStmt:
		p.stmt(s.Stmt)

	case *ast.ExprStmt:
		p.GetExprType(s.X)

	case *ast.SendStmt:
		ch := p.GetExprType(s.Chan)
		val := p.GetExprType(s.Value)
		p.assign(ch.Underlying().(*types.Chan).Elem(), val)

	case *ast.IncDecStmt:
		p.GetExprType(s.X)

	case *ast.AssignStmt:
		switch s.Tok {
		case token.ASSIGN, token.DEFINE:
			// y := x   or   y = x
			var rhsTuple types.Type
			if len(s.Lhs) != len(s.Rhs) {
				rhsTuple = p.exprN(s.Rhs[0])
			}
			for i := range s.Lhs {
				var lhs, rhs types.Type
				if rhsTuple == nil {
					rhs = p.GetExprType(s.Rhs[i]) // 1:1 assignment
				} else {
					rhs = p.extract(rhsTuple, i) // n:1 assignment
				}

				if id, ok := s.Lhs[i].(*ast.Ident); ok {
					if id.Name != "_" {
						if obj, ok := p.analyzer.TypesInfo.Defs[id]; ok {
							lhs = obj.Type() // definition
						}
					}
				}
				if lhs == nil {
					lhs = p.GetExprType(s.Lhs[i]) // assignment
				}
				p.assign(lhs, rhs)
			}

		default:
			// y op= x
			p.GetExprType(s.Lhs[0])
			p.GetExprType(s.Rhs[0])
		}

	case *ast.GoStmt:
		p.GetExprType(s.Call)

	case *ast.DeferStmt:
		p.GetExprType(s.Call)

	case *ast.ReturnStmt:
		formals := p.sig.Results()
		switch len(s.Results) {
		case formals.Len(): // 1:1
			for i, result := range s.Results {
				p.assign(formals.At(i).Type(), p.GetExprType(result))
			}

		case 1: // n:1
			tuple := p.exprN(s.Results[0])
			for i := 0; i < formals.Len(); i++ {
				p.assign(formals.At(i).Type(), p.extract(tuple, i))
			}
		}

	case *ast.SelectStmt:
		p.stmt(s.Body)

	case *ast.BlockStmt:
		for _, s := range s.List {
			p.stmt(s)
		}

	case *ast.IfStmt:
		if s.Init != nil {
			p.stmt(s.Init)
		}
		p.GetExprType(s.Cond)
		p.stmt(s.Body)
		if s.Else != nil {
			p.stmt(s.Else)
		}

	case *ast.SwitchStmt:
		if s.Init != nil {
			p.stmt(s.Init)
		}
		var tag types.Type = tUntypedBool
		if s.Tag != nil {
			tag = p.GetExprType(s.Tag)
		}
		for _, cc := range s.Body.List {
			cc := cc.(*ast.CaseClause)
			for _, cond := range cc.List {
				p.compare(tag, p.analyzer.TypesInfo.Types[cond].Type)
			}
			for _, s := range cc.Body {
				p.stmt(s)
			}
		}

	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			p.stmt(s.Init)
		}
		var I types.Type
		switch ass := s.Assign.(type) {
		case *ast.ExprStmt: // x.(type)
			I = p.GetExprType(astutil.Unparen(ass.X).(*ast.TypeAssertExpr).X)
		case *ast.AssignStmt: // y := x.(type)
			I = p.GetExprType(astutil.Unparen(ass.Rhs[0]).(*ast.TypeAssertExpr).X)
		}
		for _, cc := range s.Body.List {
			cc := cc.(*ast.CaseClause)
			for _, cond := range cc.List {
				tCase := p.analyzer.TypesInfo.Types[cond].Type
				if tCase != tUntypedNil {
					p.typeAssert(I, tCase)
				}
			}
			for _, s := range cc.Body {
				p.stmt(s)
			}
		}

	case *ast.CommClause:
		if s.Comm != nil {
			p.stmt(s.Comm)
		}
		for _, s := range s.Body {
			p.stmt(s)
		}

	case *ast.ForStmt:
		if s.Init != nil {
			p.stmt(s.Init)
		}
		if s.Cond != nil {
			p.GetExprType(s.Cond)
		}
		if s.Post != nil {
			p.stmt(s.Post)
		}
		p.stmt(s.Body)

	case *ast.RangeStmt:
		x := p.GetExprType(s.X)
		// No conversions are involved when Tok==DEFINE.
		if s.Tok == token.ASSIGN {
			if s.Key != nil {
				k := p.GetExprType(s.Key)
				var xelem types.Type
				// keys of array, *array, slice, string aren't interesting
				switch ux := x.Underlying().(type) {
				case *types.Chan:
					xelem = ux.Elem()
				case *types.Map:
					xelem = ux.Key()
				}
				if xelem != nil {
					p.assign(xelem, k)
				}
			}
			if s.Value != nil {
				val := p.GetExprType(s.Value)
				var xelem types.Type
				// values of strings aren't interesting
				switch ux := x.Underlying().(type) {
				case *types.Array:
					xelem = ux.Elem()
				case *types.Chan:
					xelem = ux.Elem()
				case *types.Map:
					xelem = ux.Elem()
				case *types.Pointer: // *array
					xelem = p.deref(ux).(*types.Array).Elem()
				case *types.Slice:
					xelem = ux.Elem()
				}
				if xelem != nil {
					p.assign(xelem, val)
				}
			}
		}
		p.stmt(s.Body)

	default:
		panic(s)
	}
}

func (p *TypeProvider) builtin(obj *types.Builtin, sig *types.Signature, args []ast.Expr, T types.Type) types.Type {
	switch obj.Name() {
	case "make", "new":
		// skip the type operand
		for _, arg := range args[1:] {
			p.GetExprType(arg)
		}

	case "append":
		s := p.GetExprType(args[0])
		if _, ok := args[len(args)-1].(*ast.Ellipsis); ok && len(args) == 2 {
			// append(x, y...)   including append([]byte, "foo"...)
			p.GetExprType(args[1])
		} else {
			// append(x, y, z)
			tElem := s.Underlying().(*types.Slice).Elem()
			for _, arg := range args[1:] {
				p.assign(tElem, p.GetExprType(arg))
			}
		}

	case "delete":
		m := p.GetExprType(args[0])
		k := p.GetExprType(args[1])
		p.assign(m.Underlying().(*types.Map).Key(), k)

	default:
		// ordinary call
		p.call(sig, args)
	}

	return T
}

func (p *TypeProvider) call(sig *types.Signature, args []ast.Expr) {
	if len(args) == 0 {
		return
	}

	// Ellipsis call?  e.g. p(x, y, z...)
	if _, ok := args[len(args)-1].(*ast.Ellipsis); ok {
		for i, arg := range args {
			// The final arg is a slice, and so is the final param.
			p.assign(sig.Params().At(i).Type(), p.GetExprType(arg))
		}
		return
	}

	var argtypes []types.Type

	// Gather the effective actual parameter types.
	if tuple, ok := p.analyzer.TypesInfo.Types[args[0]].Type.(*types.Tuple); ok {
		// p(g()) call where g has multiple results?
		p.GetExprType(args[0])
		// unpack the tuple
		for i := 0; i < tuple.Len(); i++ {
			argtypes = append(argtypes, tuple.At(i).Type())
		}
	} else {
		for _, arg := range args {
			argtypes = append(argtypes, p.GetExprType(arg))
		}
	}

	// Assign the actuals to the formals.
	if !sig.Variadic() {
		for i, argtype := range argtypes {
			p.assign(sig.Params().At(i).Type(), argtype)
		}
	} else {
		// The first n-1 parameters are assigned normally.
		nnormals := sig.Params().Len() - 1
		for i, argtype := range argtypes[:nnormals] {
			p.assign(sig.Params().At(i).Type(), argtype)
		}
		// Remaining args are assigned to elements of varargs slice.
		tElem := sig.Params().At(nnormals).Type().(*types.Slice).Elem()
		for i := nnormals; i < len(argtypes); i++ {
			p.assign(tElem, argtypes[i])
		}
	}
}

func (p *TypeProvider) compare(x, y types.Type) {
	if types.AssignableTo(x, y) {
		p.assign(y, x)
	} else if types.AssignableTo(y, x) {
		p.assign(x, y)
	}
}

// exprN visits an expression in a multi-value context.
func (p *TypeProvider) exprN(e ast.Expr) types.Type {
	typ := p.analyzer.TypesInfo.Types[e].Type.(*types.Tuple)
	switch e := e.(type) {
	case *ast.ParenExpr:
		return p.exprN(e.X)

	case *ast.CallExpr:
		// x, err := p(args)
		sig := p.GetExprType(e.Fun).Underlying().(*types.Signature)
		p.call(sig, e.Args)

	case *ast.IndexExpr:
		// y, ok := x[i]
		x := p.GetExprType(e.X)
		p.assign(p.GetExprType(e.Index), x.Underlying().(*types.Map).Key())

	case *ast.TypeAssertExpr:
		// y, ok := x.(T)
		p.typeAssert(p.GetExprType(e.X), typ.At(0).Type())

	case *ast.UnaryExpr: // must be receive <-
		// y, ok := <-x
		p.GetExprType(e.X)

	default:
		panic(e)
	}
	return typ
}

func (p *TypeProvider) extract(tuple types.Type, i int) types.Type {
	if tuple, ok := tuple.(*types.Tuple); ok && i < tuple.Len() {
		return tuple.At(i).Type()
	}
	return tInvalid
}

func (p *TypeProvider) valueSpec(spec *ast.ValueSpec) {
	var T types.Type
	if spec.Type != nil {
		T = p.analyzer.TypesInfo.Types[spec.Type].Type
	}
	switch len(spec.Values) {
	case len(spec.Names): // e.g. var x, y = f(), g()
		for _, value := range spec.Values {
			v := p.GetExprType(value)
			if T != nil {
				p.assign(T, v)
			}
		}

	case 1: // e.g. var x, y = f()
		tuple := p.exprN(spec.Values[0])
		for i := range spec.Names {
			if T != nil {
				p.assign(T, p.extract(tuple, i))
			}
		}
	}
}

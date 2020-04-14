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

// Package datalog implements a datalog prover.
//
// This package is based on a C and Lua library found at:
//
//   http://www.ccs.neu.edu/home/ramsdell/tools/datalog/
//
// My thanks to John D. Ramsdell, the author of that library for permission to
// distribute this new work under an Apache License. For reference, John
// Ramsdell's original code includes the following copyright and license notice:
//
//   Datalog 2.4
//
//   A small Datalog interpreter written in Lua designed to be used via a
//   simple C API.
//
//   John D. Ramsdell
//   Copyright (C) 2004 The MITRE Corporation
//
//   This library is free software; you can redistribute it and/or modify
//   it under the terms of the GNU Lesser General Public License as
//   published by the Free Software Foundation; either version 2 of the
//   License, or (at your option) any later version.
//
//   This library is distributed in the hope that it will be useful, but
//   WITHOUT ANY WARRANTY; without even the implied warranty of
//   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
//   Lesser General Public License for more details.
//
//   You should have received a copy of the GNU Lesser General Public
//   License along with this library; if not, write to the Free Software
//   Foundation, Inc.  51 Franklin St, Fifth Floor, Boston, MA 02110-1301
//   USA
package datalog

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
)

// Notes on uniqueness: The datalog engine must be able to tell when two
// variables are the "same". When variables are represented by distinct textual
// names, like "X" or "Y", this is trivial: just compare the text. This applies
// to constants, identifiers, and predicate symbols as well.
//
// As an optimization, the Lua implementation interns all variables (and
// identifiers, etc.) before processing. This step requires that: (1) each
// variable can be used as the key to a map; and (2) a variable can be stored as
// a value in a map without preventing garbage collection. The Lua
// implementation solves (1) using the textual names, and solves (2) using maps
// with weak references.
//
// All of the above is problematic in go. First, distinct textual names are only
// readily available when processing datalog written in text. When datalog is
// driven programmatically, assigning distinct textual names is a bother.
// Second, many values in go can't be used as keys in a map. In particular,
// literals can't be, since these are structs that contain slices. Finally, go
// doesn't provide weak references, so the typical approach to interning using a
// map would lead to garbage collection issues.
//
// This implementation uses a different approach. It allows a variety of pointer
// types to be used as variables, identifiers, constants, or predicate symbols.
// Two variables (etc.) are then considered the "same" if the pointers are
// equal, i.e. if they point to the same go object. It is the caller's
// responsibility to ensure that the same go object is used when the same
// variable is intended.
//
// There is a wrinkle, however: there is no way in go to express a constraint
// that only pointer types can be used as variables. We work around this by
// requiring variables to embed an anonymous Var struct. Only a pointer to [a
// struct containing] Var can be used as a variable.

// id is used to distinguish different variables, constants, etc.
type id uintptr

// Const represents a concrete datalog value that can be used as a term. Typical
// examples include alice, bob, "Hello", 42, -3, and other printable sequences.
// This implementation doesn't place restrictions on the contents.
type Const interface {
	// cID returns a distinct number for each live Const.
	cID() id
	Term
}

// DistinctConst can be embedded as an anonymous field in a struct T, enabling
// *T to be used as a Const.
type DistinctConst struct {
	_ byte // avoid confounding pointers due to zero size
}

// String for a DistinctConst prints the internal ID. This should be
// hidden by types T in which DistinctConst is embedded.
func (c *DistinctConst) String() string {
	return fmt.Sprintf("Const{0x%x}", c.cID())
}

func (c *DistinctConst) cID() id {
	return id(reflect.ValueOf(c).Pointer())
}

// Constant returns true for all objects that embed DistinctConst.
func (c *DistinctConst) Constant() bool {
	return true
}

// Variable returns false for all objects that embed DistinctConst.
func (c *DistinctConst) Variable() bool {
	return false
}

// Var represents a datalog variable. These are typically written with initial
// uppercase, e.g. X, Y, Left_child. This implementation doesn't restrict or
// even require variable names.
type Var interface {
	// vID returns a distinct number for each live Var.
	vID() id
	Term
}

// DistinctVar can be embedded as an anonymous field in a struct T, enabling *T
// to be used as a Var. In addition, &DistinctVar{} can be used as a fresh Var
// that has no name or associated data but is distinct from all other live Vars.
type DistinctVar struct {
	_ byte // avoid confounding pointers due to zero size
}

// String for a DistinctVar prints the internal ID. This should be
// hidden by types T in which DistinctVar is embedded.
func (v *DistinctVar) String() string {
	return fmt.Sprintf("Var{0x%x}", v.vID())
}

func (v *DistinctVar) vID() id {
	return id(reflect.ValueOf(v).Pointer())
}

// Constant returns false for all objects that embed DistinctVariable.
func (v *DistinctVar) Constant() bool {
	return false
}

// Variable returns true for all objects that embed DistinctVariable.
func (v *DistinctVar) Variable() bool {
	return true
}

// Term represents an argument of a literal. Var and Const implement Term.
type Term interface {
	// Constant checks whether this term is a Const. Same as _, ok := t.(Const).
	Constant() bool

	// Variable checks whether this term is a Var. Same as _, ok := t.(Var).
	Variable() bool
}

// Literal represents a predicate with terms for arguments. Typical examples
// include person(alice), ancestor(alice, bob), and ancestor(eve, X).
type Literal struct {
	Pred      Pred
	Arg       []Term
	cachedTag *string
}

// NewLiteral returns a new literal with the given predicate and arguments. The
// number of arguments must match the predicate's arity, else panic ensues.
func NewLiteral(p Pred, arg ...Term) *Literal {
	if p.Arity() != len(arg) {
		panic("datalog: arity mismatch")
	}
	return &Literal{Pred: p, Arg: arg}
}

// String is a pretty-printer for literals. It produces traditional datalog
// syntax, assuming that all the predicates and terms do when printed with %v.
func (l *Literal) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v", l.Pred)
	if len(l.Arg) > 0 {
		fmt.Fprintf(&buf, "(%v", l.Arg[0])
		for i := 1; i < len(l.Arg); i++ {
			fmt.Fprintf(&buf, ", %v", l.Arg[i])
		}
		fmt.Fprintf(&buf, ")")
	}
	return buf.String()
}

// tag returns a "variant tag" for a literal, such that two literals have the
// same variant tag if and only if they are identical modulo variable renaming.
func (l *Literal) tag() string {
	if l.cachedTag != nil {
		return *l.cachedTag
	}
	var buf bytes.Buffer
	l.tagf(&buf, make(map[id]int))
	tag := buf.String()
	l.cachedTag = &tag
	return tag
}

// tagf writes a literal's "variant tag" into buf after renaming variables
// according to the varNum map. If the varNum map is nil, then variables are not
// renamed.
func (l *Literal) tagf(buf *bytes.Buffer, varNum map[id]int) {
	// Tag encoding: hex(pred-id),term,term,...
	// with varMap, term consts are hex, term vars are "v0", "v1", ...
	// with no varMap, terms are all hex
	fmt.Fprintf(buf, "%x", l.Pred.pID())
	for _, arg := range l.Arg {
		switch arg := arg.(type) {
		case Const:
			fmt.Fprintf(buf, ",%x", arg.cID())
		case Var:
			vid := arg.vID()
			num, ok := varNum[vid]
			if !ok {
				num = len(varNum)
				varNum[vid] = num
			}
			fmt.Fprintf(buf, ",v%d", num)
		default:
			panic("datalog: not reached -- term is always Var or Const")
		}
	}
}

// Clause has a head literal and zero or more body literals. With an empty
// body, it is known as a fact. Otherwise, a rule.
// Example fact: parent(alice, bob)
// Example rule: ancestor(A, C) :- ancestor(A, B), ancestor(B, C)
type Clause struct {
	Head *Literal
	Body []*Literal
}

// NewClause constructs a new fact (if there are no arguments) or rule
// (otherwise).
func NewClause(head *Literal, body ...*Literal) *Clause {
	return &Clause{Head: head, Body: body}
}

// String is a pretty-printer for clauses. It produces traditional datalog
// syntax, assuming that all the predicates and terms do when printed with %v.
func (c *Clause) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s", c.Head.String())
	if len(c.Body) > 0 {
		fmt.Fprintf(&buf, " :- %s", c.Body[0].String())
		for i := 1; i < len(c.Body); i++ {
			fmt.Fprintf(&buf, ", %s", c.Body[i].String())
		}
	}
	return buf.String()
}

// Pred represents a logical predicate, or relation, of a given arity.
type Pred interface {
	// pID returns a distinct number for each live Pred.
	pID() id

	// Arity returns the arity of the predicate, i.e. the number of arguments it
	// takes.
	Arity() int

	// Assert introduces new information about a predicate. Assert is only
	// called by the prover for Pred p if clause is safe and p == c.Head.Pred.
	Assert(clause *Clause) error

	// Retract removes information about a predicate. Retract is only called by
	// the prover for Pred p if p == c.Head.Pred.
	Retract(clause *Clause) error

	// Search is called by the prover to discover information about a predicate.
	// For each fact or rule whose head unifies with the target, Search should
	// call the given callback.
	Search(target *Literal, discovered func(c *Clause))
}

// DistinctPred can be embedded as an anonymous field in a struct T, enabling
// *T to be used as a Pred.
type DistinctPred struct {
	// WithArity is the arity of the predicate. The field name Arity can not be
	// used, since it clashes with the function of the same name. The prefix
	// "With" was chosen to accommodate this syntax:
	//   p := &DistinctPred{ WithArity: 3 }
	WithArity int
}

// String for a DistinctPred prints the internal ID and the arity. This should
// be hidden by types T in which Distinct and the arityConst is embedded.
func (p *DistinctPred) String() string {
	return fmt.Sprintf("Pred{0x%x}/%d", p.pID(), p.Arity())
}

func (p *DistinctPred) pID() id {
	return id(reflect.ValueOf(p).Pointer())
}

// Arity returns the arity of the predicate, i.e. the number of arguments it
// takes.
func (p *DistinctPred) Arity() int {
	return p.WithArity
}

// SetArity sets the arity of the predicate, i.e. the number of arguments it
// takes. This function should be called only early when initializing a new Pred
// object. This function exists only to avoid ugly compound initializers like:
//   p := &datalog.DBPred{datalog.DistinctPred:datalog.DistinctPred{WithArity: 2}}
// Instead, one can do:
//   p := new(datalog.DBPred)
//   p.SetArity(2)
func (p *DistinctPred) SetArity(arity int) {
	p.WithArity = arity
}

// DBPred holds a predicate that is defined by a database of facts and rules.
type DBPred struct {
	db []*Clause
	DistinctPred
}

// Assert checks if the clause is safe then calls Assert() on the appropriate
// Pred.
func (c *Clause) Assert() error {
	if !c.Safe() {
		return errors.New("datalog: can't assert unsafe clause")
	}
	return c.Head.Pred.Assert(c)
}

// Assert for a DBPred inserts c into the database for this predicate.
func (p *DBPred) Assert(c *Clause) error {
	p.db = append(p.db, c)
	return nil
}

// tag returns a "variant tag" for a clause, such that two clauses have the
// same variant tag if and only if they are identical modulo variable renaming.
func (c *Clause) tag() string {
	var buf bytes.Buffer
	varMap := make(map[id]int)
	c.Head.tagf(&buf, varMap)
	for _, literal := range c.Body {
		literal.tagf(&buf, varMap)
	}
	return buf.String()
}

// Retract calls Retract() on the appropriate Pred.
func (c *Clause) Retract() error {
	return c.Head.Pred.Retract(c)
}

// Retract for a DBPred removes a clause from the relevant database, along with
// all structurally identical clauses modulo variable renaming.
func (p *DBPred) Retract(c *Clause) error {
	tag := c.tag()
	for i := 0; i < len(p.db); i++ {
		if p.db[i].tag() == tag {
			n := len(p.db)
			p.db[i], p.db[n-1], p.db = p.db[n-1], nil, p.db[:n-1]
			i--
		}
	}
	return nil
}

// Answers to a query are facts.
type Answers []*Literal

// String is a pretty-printer for Answers. It produces traditional datalog
// syntax, assuming that all the predicates and terms do when printed with %v.
func (a Answers) String() string {
	if len(a) == 0 {
		return "% empty"
	} else if len(a) == 1 {
		return a[0].String() + "."
	} else {
		var buf bytes.Buffer
		for _, fact := range a {
			fmt.Fprintf(&buf, "%s.\n", fact.String())
		}
		return buf.String()
	}
}

// Query returns a list of facts that unify with the given literal.
func (l *Literal) Query() Answers {
	facts := make(query).search(l).facts
	if len(facts) == 0 {
		return nil
	}
	a := make(Answers, len(facts))
	i := 0
	for _, fact := range facts {
		a[i] = fact
		i++
	}
	return a
}

// An env maps variables to terms. It is used for substitutions.
type env map[Var]Term

// subst creates a new literal by applying env.
func (l *Literal) subst(e env) *Literal {
	if e == nil || len(e) == 0 || len(l.Arg) == 0 {
		return l
	}
	s := &Literal{Pred: l.Pred, Arg: make([]Term, len(l.Arg))}
	copy(s.Arg, l.Arg)
	for i, arg := range l.Arg {
		if v, ok := arg.(Var); ok {
			if t, ok := e[v]; ok {
				s.Arg[i] = t
			}
		}
	}
	return s
}

// shuffle extends env by adding, for each unmapped variable in the literal's
// arguments, a mappings to a fresh variable. If env is nil, a new environment
// is created.
func (l *Literal) shuffle(e env) env {
	if e == nil {
		e = make(env)
	}
	for _, arg := range l.Arg {
		if v, ok := arg.(Var); ok {
			if _, ok := e[v]; !ok {
				e[v] = &DistinctVar{}
			}
		}
	}
	return e
}

// rename generates a new literal by renaming all variables to fresh ones.
func (l *Literal) rename() *Literal {
	return l.subst(l.shuffle(nil))
}

// chase applies env to a term until a constant or an unmapped variable is reached.
func chase(t Term, e env) Term {
	for {
		v, ok := t.(Var)
		if !ok {
			break
		}
		next, ok := e[v]
		if !ok {
			break
		}
		t = next
	}
	return t
}

// unify two terms, where a != b
func unifyTerms(a, b Term, e env) env {
	if va, ok := a.(Var); ok {
		if vb, ok := b.(Var); ok {
			// unify var var
			e[va] = vb
		} else {
			// unify var const
			e[va] = b
		}
	} else {
		if vb, ok := b.(Var); ok {
			// unify const var
			e[vb] = a
		} else {
			// unify const const
			return nil
		}
	}
	return e
}

// unify attempts to unify two literals. It returns an environment such that
// a.subst(env) is structurally identical to b.subst(env), or nil if no such
// environment is possible.
func unify(a, b *Literal) env {
	if a.Pred != b.Pred {
		return nil
	}
	e := make(env)
	for i := range a.Arg {
		aT := chase(a.Arg[i], e)
		bT := chase(b.Arg[i], e)
		if aT != bT {
			e = unifyTerms(aT, bT, e)
			if e == nil {
				return nil
			}
		}
	}
	return e
}

// drop creates a new clause by dropping d leading parts from the body, then
// applying env to head and to each remaining body part. Caller must ensure
// len(c.Body) >= d.
func (c *Clause) drop(d int, e env) *Clause {
	n := len(c.Body) - d
	s := &Clause{
		Head: c.Head.subst(e),
		Body: make([]*Literal, n),
	}
	for i := 0; i < n; i++ {
		s.Body[i] = c.Body[i+d].subst(e)
	}
	return s
}

// subst creates a new clause by applying env to head and to each body part
func (c *Clause) subst(e env) *Clause {
	if e == nil || len(e) == 0 {
		return c
	}
	return c.drop(0, e)
}

// rename generates a new clause by renaming all variables to freshly created
// variables.
func (c *Clause) rename() *Clause {
	// Note: since all variables in head are also in body, we can ignore head
	// while generating the environment.
	var e env
	for _, part := range c.Body {
		e = part.shuffle(e)
	}
	return c.subst(e)
}

// hasVar checks if v appears in a literal.
func (l *Literal) hasVar(v Var) bool {
	for _, arg := range l.Arg {
		if v == arg {
			return true
		}
	}
	return false
}

// Safe checks whether a clause is safe, that is, whether every variable in the
// head also appears in the body.
func (c *Clause) Safe() bool {
	for _, arg := range c.Head.Arg {
		if v, ok := arg.(Var); ok {
			safe := false
			for _, literal := range c.Body {
				if literal.hasVar(v) {
					safe = true
					break
				}
			}
			if !safe {
				return false
			}
		}
	}
	return true
}

// The remainder of this file implements the datalog prover.

// query tracks a set of subgoals, indexed by subgoal target tag.
type query map[string]*subgoal

// newSubgoal creates a new subgoal and adds it to the query's subgoal set.
func (q query) newSubgoal(target *Literal, waiters []*waiter) *subgoal {
	sg := &subgoal{target, make(factSet), waiters}
	q[target.tag()] = sg
	return sg
}

// findSubgoal returns the appropriate subgoal from the query's subgoal set.
func (q query) findSubgoal(target *Literal) *subgoal {
	return q[target.tag()]
}

// factSet tracks a set of facts, indexed by tag.
type factSet map[string]*Literal

type subgoal struct {
	target  *Literal  // e.g. ancestor(X, Y)
	facts   factSet   // facts that unify with target, e.g. ancestor(alice, bob)
	waiters []*waiter // waiters such that target unifies with waiter.rule.body[0]
}

// waiter is a (subgoal, rule) pair, where rule.head unifies with
// subgoal.target.
type waiter struct {
	subgoal *subgoal
	rule    *Clause
}

// search introduces a new subgoal for target, with waiters to be notified upon
// discovery of new facts that unify with target.
// Example target: ancestor(X, Y)
func (q query) search(target *Literal, waiters ...*waiter) *subgoal {
	sg := q.newSubgoal(target, waiters)
	target.Pred.Search(target, func(c *Clause) {
		q.discovered(sg, c)
	})
	return sg
}

// Search for DBPred examines facts and rules in the database for this predicate
// and, if the clause head unifies with the target, reports the discovery.
func (p *DBPred) Search(target *Literal, discovered func(c *Clause)) {
	// Examine each fact or rule clause in the relevant database ...
	// Example fact: ancestor(alice, bob)
	// Example rule: ancestor(P, Q) :- parent(P, Q)
	for _, clause := range p.db {
		// ... and try to unify target with that clause's head.
		renamed := clause.rename()
		e := unify(target, renamed.Head)
		if e != nil {
			// Upon success, process the new discovery.
			discovered(renamed.subst(e))
		}
	}
}

// discovered kicks off processing upon discovery of a fact or rule clause
// whose head unifies with a subgoal target.
func (q query) discovered(sg *subgoal, clause *Clause) {
	if len(clause.Body) == 0 {
		q.discoveredFact(sg, clause.Head)
	} else {
		q.discoveredRule(sg, clause)
	}
}

// discoveredRule kicks off processing upon discovery of a rule whose head
// unifies with a subgoal target.
func (q query) discoveredRule(rulesg *subgoal, rule *Clause) {
	bodysg := q.findSubgoal(rule.Body[0])
	if bodysg == nil {
		// Nothing on body[0], so search for it, but resume processing later.
		q.search(rule.Body[0], &waiter{rulesg, rule})
	} else {
		// Work is progress on body[0], so resume processing later...
		bodysg.waiters = append(bodysg.waiters, &waiter{rulesg, rule})
		// ... but also check facts already known to unify with body[0]. For each
		// such fact, check if rule can be simplified using information from fact.
		// If so then we have discovered a new, simpler rule whose head unifies with
		// the rulesg.target.
		var simplifiedRules []*Clause
		for _, fact := range bodysg.facts {
			r := resolve(rule, fact)
			if r != nil {
				simplifiedRules = append(simplifiedRules, r)
			}
		}
		for _, r := range simplifiedRules {
			q.discovered(rulesg, r)
		}
	}
}

// discoveredRule kicks off processing upon discovery of a fact that unifies
// with a subgoal target.
func (q query) discoveredFact(factsg *subgoal, fact *Literal) {
	if _, ok := factsg.facts[fact.tag()]; !ok {
		factsg.facts[fact.tag()] = fact
		// Resume processing: For each deferred (rulesg, rule) pair, check if rule
		// can be simplified using information from fact. If so then we have
		// discovered a new, simpler rule whose head unifies with rulesg.target.
		for _, waiting := range factsg.waiters {
			r := resolve(waiting.rule, fact)
			if r != nil {
				q.discovered(waiting.subgoal, r)
			}
		}
	}
}

// resolve simplifies rule using information from fact.
// Example rule:    ancestor(X, Z) :- ancestor(X, Y), ancestor(Y, Z)
// Example fact:    ancestor(alice, bob)
// Simplified rule: ancestor(alice, Z) :- ancestor(bob, Z)
func resolve(rule *Clause, fact *Literal) *Clause {
	if len(rule.Body) == 0 {
		panic("datalog: not reached -- rule can't have empty body")
	}
	if fact.rename() != fact {
		panic("datalog: not reached -- fact should not have variables")
	}
	e := unify(rule.Body[0], fact)
	if e == nil {
		return nil
	}
	return rule.drop(1, e)
}

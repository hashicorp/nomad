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

// Package dlprim provides custom "primitive" datalog predicates, like Equals.
package dlprim

import (
	"errors"

	"github.com/kevinawalsh/datalog"
)

// Equals is a custom predicate for equality checking, defined by these rules:
//   =(X, Y) generates no facts.
//   =(X, c) generates fact =(c, c).
//   =(c, Y) generates fact =(c, c).
//   =(c, c) generates fact =(c, c).
//   =(c1, c2) generates no facts.
var Equals datalog.Pred

func init() {
	eq := new(eqPrim)
	eq.SetArity(2)
	Equals = eq
}

type eqPrim struct {
	datalog.DistinctPred
}

func (eq *eqPrim) String() string {
	return "="
}

func (eq *eqPrim) Assert(c *datalog.Clause) error {
	return errors.New("datalog: can't assert for custom predicates")
}

func (eq *eqPrim) Retract(c *datalog.Clause) error {
	return errors.New("datalog: can't retract for custom predicates")
}

func (eq *eqPrim) Search(target *datalog.Literal, discovered func(c *datalog.Clause)) {
	a := target.Arg[0]
	b := target.Arg[1]
	if a.Variable() && b.Constant() {
		discovered(datalog.NewClause(datalog.NewLiteral(eq, b, b)))
	} else if a.Constant() && b.Variable() {
		discovered(datalog.NewClause(datalog.NewLiteral(eq, a, a)))
	} else if a.Constant() && b.Constant() && a == b {
		discovered(datalog.NewClause(target))
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Iterator is the interface that must be implemented to supply data to the
// Paginator.
type Iterator interface {
	// Next returns the next element to be considered for pagination.
	// The page will end if nil is returned.
	Next() interface{}
}

// Paginator wraps an iterator and returns only the expected number of pages.
type Paginator[T, TStub any] struct {
	iter           Iterator
	tokenizer      Tokenizer[T]
	bexpr          *bexpr.Evaluator
	filter         FilterFunc[T]
	stubFn         func(T) (TStub, error)
	perPage        int32
	itemCount      int32
	nextToken      string
	reverse        bool
	nextTokenFound bool
	pageErr        error
}

// NewPaginator returns a new Paginator. Any error creating the paginator is
// due to bad user filter input, RPC functions should therefore return a 400
// error code along with an appropriate message.
func NewPaginator[T, TStub any](
	iter Iterator,
	opts structs.QueryOptions,
	tokenizer Tokenizer[T],
	stubFn func(T) (TStub, error),
) (*Paginator[T, TStub], error) {

	p := &Paginator[T, TStub]{
		iter:           iter,
		tokenizer:      tokenizer,
		stubFn:         stubFn,
		perPage:        opts.PerPage,
		reverse:        opts.Reverse,
		nextTokenFound: opts.NextToken == "",
	}

	if opts.Filter != "" {
		evaluator, err := bexpr.CreateEvaluator(opts.Filter)
		if err != nil {
			return nil, fmt.Errorf("failed to read filter expression: %v", err)
		}
		p.bexpr = evaluator
	}

	return p, nil
}

func (p *Paginator[T, TStub]) WithFilter(fn FilterFunc[T]) *Paginator[T, TStub] {
	p.filter = fn
	return p
}

// Page populates a page by running the append function
// over all results. Returns the next token.
func (p *Paginator[T, TStub]) Page() ([]TStub, string, error) {
	out := []TStub{}
DONE:
	for {
		obj, andThen := p.next()
		switch andThen {
		case paginatorInclude:
			out = append(out, obj)
		case paginatorSkip:
			continue
		case paginatorComplete:
			break DONE
		}
	}
	return out, p.nextToken, p.pageErr
}

func (p *Paginator[T, TStub]) next() (TStub, paginatorState) {
	var none TStub
	raw := p.iter.Next()
	if raw == nil {
		p.nextToken = ""
		return none, paginatorComplete
	}
	obj, ok := raw.(T)
	if !ok {
		p.pageErr = errors.New("paginator was instantiated with wrong type for table")
		return none, paginatorComplete
	}

	token, compared := p.tokenizer(obj)
	p.nextToken = token

	var passedToken bool
	if p.reverse {
		passedToken = compared == 1 // token > target token
	} else {
		passedToken = compared == -1 // token < target token
	}

	if !p.nextTokenFound && passedToken {
		return none, paginatorSkip
	}

	if p.bexpr != nil {
		allow, err := p.bexpr.Evaluate(raw)
		if err != nil {
			p.pageErr = err
			return none, paginatorComplete
		}
		if !allow {
			return none, paginatorSkip
		}
	}

	if p.filter != nil && !p.filter(obj) {
		return none, paginatorSkip
	}

	p.nextTokenFound = true
	out, err := p.stubFn(obj)
	if err != nil {
		p.pageErr = err
		return none, paginatorComplete
	}

	// have we produced enough results for this page?
	p.itemCount++
	if p.perPage != 0 && p.itemCount > p.perPage {
		return out, paginatorComplete
	}

	return out, paginatorInclude
}

type paginatorState int

const (
	paginatorInclude paginatorState = iota
	paginatorSkip
	paginatorComplete
)

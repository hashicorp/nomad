// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
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
type Paginator[T any] struct {
	iter           Iterator
	tokenizer      Tokenizer[T]
	filters        []Filter
	perPage        int32
	itemCount      int32
	nextToken      string
	reverse        bool
	nextTokenFound bool
	pageErr        error

	// appendFunc is the function the caller should use to append raw
	// entries to the results set. The object is guaranteed to be
	// non-nil.
	appendFunc func(interface{}) error
}

// NewPaginator returns a new Paginator. Any error creating the paginator is
// due to bad user filter input, RPC functions should therefore return a 400
// error code along with an appropriate message.
func NewPaginator[T any](iter Iterator, tokenizer Tokenizer[T], filters []Filter,
	opts structs.QueryOptions, appendFunc func(interface{}) error) (*Paginator[T], error) {

	var evaluator *bexpr.Evaluator
	var err error

	if opts.Filter != "" {
		evaluator, err = bexpr.CreateEvaluator(opts.Filter)
		if err != nil {
			return nil, fmt.Errorf("failed to read filter expression: %v", err)
		}
		filters = append(filters, evaluator)
	}

	return &Paginator[T]{
		iter:           iter,
		tokenizer:      tokenizer,
		filters:        filters,
		perPage:        opts.PerPage,
		reverse:        opts.Reverse,
		nextTokenFound: opts.NextToken == "",
		appendFunc:     appendFunc,
	}, nil
}

// Page populates a page by running the append function
// over all results. Returns the next token.
func (p *Paginator[T]) Page() (string, error) {
DONE:
	for {
		raw, andThen := p.next()
		switch andThen {
		case paginatorInclude:
			err := p.appendFunc(raw)
			if err != nil {
				p.pageErr = err
				break DONE
			}
		case paginatorSkip:
			continue
		case paginatorComplete:
			break DONE
		}
	}
	return p.nextToken, p.pageErr
}

func (p *Paginator[T]) next() (interface{}, paginatorState) {
	raw := p.iter.Next()
	if raw == nil {
		p.nextToken = ""
		return nil, paginatorComplete
	}
	obj, ok := raw.(T)
	if !ok {
		panic("paginator was instantiated with wrong type for table")
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
		return nil, paginatorSkip
	}

	// apply filters if defined
	for _, f := range p.filters {
		allow, err := f.Evaluate(raw)
		if err != nil {
			p.pageErr = err
			return nil, paginatorComplete
		}
		if !allow {
			return nil, paginatorSkip
		}
	}

	p.nextTokenFound = true

	// have we produced enough results for this page?
	p.itemCount++
	if p.perPage != 0 && p.itemCount > p.perPage {
		return raw, paginatorComplete
	}

	return raw, paginatorInclude
}

type paginatorState int

const (
	paginatorInclude paginatorState = iota
	paginatorSkip
	paginatorComplete
)

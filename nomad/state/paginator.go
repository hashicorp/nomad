package state

import (
	"fmt"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Iterator is the interface that must be implemented to use the Paginator.
type Iterator interface {
	// Next returns the next element to be considered for pagination along with
	// a token string used to uniquely identify elements in the iteration.
	// The page will end if nil is returned.
	// Tokens should have a stable order and the order must match the paginator
	// ascending property.
	Next() (string, interface{})
}

// Paginator is an iterator over a memdb.ResultIterator that returns
// only the expected number of pages.
type Paginator struct {
	iter           Iterator
	perPage        int32
	itemCount      int32
	seekingToken   string
	nextToken      string
	ascending      bool
	nextTokenFound bool
	pageErr        error

	// filterEvaluator is used to filter results using go-bexpr. It's nil if
	// no filter expression is defined.
	filterEvaluator *bexpr.Evaluator

	// appendFunc is the function the caller should use to append raw
	// entries to the results set. The object is guaranteed to be
	// non-nil.
	appendFunc func(interface{}) error
}

func NewPaginator(iter Iterator, opts structs.QueryOptions, appendFunc func(interface{}) error) (*Paginator, error) {
	var evaluator *bexpr.Evaluator
	var err error

	if opts.Filter != "" {
		evaluator, err = bexpr.CreateEvaluator(opts.Filter)
		if err != nil {
			return nil, fmt.Errorf("failed to read filter expression: %v", err)
		}
	}

	return &Paginator{
		iter:            iter,
		perPage:         opts.PerPage,
		seekingToken:    opts.NextToken,
		ascending:       opts.Ascending,
		nextTokenFound:  opts.NextToken == "",
		filterEvaluator: evaluator,
		appendFunc:      appendFunc,
	}, nil
}

// Page populates a page by running the append function
// over all results. Returns the next token
func (p *Paginator) Page() (string, error) {
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

func (p *Paginator) next() (interface{}, paginatorState) {
	token, raw := p.iter.Next()
	if raw == nil {
		p.nextToken = ""
		return nil, paginatorComplete
	}

	// have we found the token we're seeking (if any)?
	p.nextToken = token

	var passedToken bool
	if p.ascending {
		passedToken = token < p.seekingToken
	} else {
		passedToken = token > p.seekingToken
	}

	if !p.nextTokenFound && passedToken {
		return nil, paginatorSkip
	}

	// apply filter if defined
	if p.filterEvaluator != nil {
		match, err := p.filterEvaluator.Evaluate(raw)
		if err != nil {
			p.pageErr = err
			return nil, paginatorComplete
		}
		if !match {
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

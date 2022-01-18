package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Paginator is an iterator over a memdb.ResultIterator that returns
// only the expected number of pages.
type Paginator struct {
	iter           memdb.ResultIterator
	perPage        int32
	itemCount      int32
	seekingToken   string
	nextToken      string
	nextTokenFound bool

	// appendFunc is the function the caller should use to append raw
	// entries to the results set. The object is guaranteed to be
	// non-nil.
	appendFunc func(interface{})
}

func NewPaginator(iter memdb.ResultIterator, opts structs.QueryOptions, appendFunc func(interface{})) *Paginator {
	return &Paginator{
		iter:           iter,
		perPage:        opts.PerPage,
		seekingToken:   opts.NextToken,
		nextTokenFound: opts.NextToken == "",
		appendFunc:     appendFunc,
	}
}

// Page populates a page by running the append function
// over all results. Returns the next token
func (p *Paginator) Page() string {
DONE:
	for {
		raw, andThen := p.next()
		switch andThen {
		case paginatorInclude:
			p.appendFunc(raw)
		case paginatorSkip:
			continue
		case paginatorComplete:
			break DONE
		}
	}
	return p.nextToken
}

func (p *Paginator) next() (interface{}, paginatorState) {
	raw := p.iter.Next()
	if raw == nil {
		p.nextToken = ""
		return nil, paginatorComplete
	}

	// have we found the token we're seeking (if any)?
	id := raw.(IDGetter).GetID()
	p.nextToken = id
	if !p.nextTokenFound && id < p.seekingToken {
		return nil, paginatorSkip
	}
	p.nextTokenFound = true

	// have we produced enough results for this page?
	p.itemCount++
	if p.perPage != 0 && p.itemCount > p.perPage {
		return raw, paginatorComplete
	}

	return raw, paginatorInclude
}

// IDGetter must be implemented for the results of any iterator we
// want to paginate
type IDGetter interface {
	GetID() string
}

type paginatorState int

const (
	paginatorInclude paginatorState = iota
	paginatorSkip
	paginatorComplete
)

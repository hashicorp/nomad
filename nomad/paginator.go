package nomad

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
}

func newPaginator(iter memdb.ResultIterator, opts structs.QueryOptions) *Paginator {
	return &Paginator{
		iter:           iter,
		perPage:        opts.PerPage,
		seekingToken:   opts.NextToken,
		nextTokenFound: opts.NextToken == "",
	}
}

func (p *Paginator) Next() (interface{}, paginatorState) {
	raw := p.iter.Next()
	if raw == nil {
		p.nextToken = ""
		return nil, PaginatorComplete
	}

	// have we found the token we're seeking (if any)?
	id := raw.(IDGetter).GetID()
	p.nextToken = id
	if !p.nextTokenFound && id < p.seekingToken {
		return nil, PaginatorSkip
	}
	p.nextTokenFound = true

	// have we produced enough results for this page?
	p.itemCount++
	if p.perPage != 0 && p.itemCount > p.perPage {
		return raw, PaginatorComplete
	}

	return raw, PaginatorInclude
}

func (p *Paginator) NextToken() string {
	return p.nextToken
}

// IDGetter must be implemented for the results of any iterator we
// want to paginate
type IDGetter interface {
	GetID() string
}

type paginatorState int

const (
	PaginatorInclude paginatorState = iota
	PaginatorSkip
	PaginatorComplete
)

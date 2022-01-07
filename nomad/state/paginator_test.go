package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestPaginator(t *testing.T) {
	t.Parallel()
	ids := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	cases := []struct {
		name              string
		perPage           int32
		nextToken         string
		expected          []string
		expectedNextToken string
	}{
		{
			name:              "size-3 page-1",
			perPage:           3,
			expected:          []string{"0", "1", "2"},
			expectedNextToken: "3",
		},
		{
			name:              "size-5 page-2 stop before end",
			perPage:           5,
			nextToken:         "3",
			expected:          []string{"3", "4", "5", "6", "7"},
			expectedNextToken: "8",
		},
		{
			name:              "page-2 reading off the end",
			perPage:           10,
			nextToken:         "5",
			expected:          []string{"5", "6", "7", "8", "9"},
			expectedNextToken: "",
		},
		{
			name:              "starting off the end",
			perPage:           5,
			nextToken:         "a",
			expected:          []string{},
			expectedNextToken: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			iter := newTestIterator(ids)
			results := []string{}

			paginator := NewPaginator(iter,
				structs.QueryOptions{
					PerPage: tc.perPage, NextToken: tc.nextToken,
				},
				func(raw interface{}) {
					result := raw.(*mockObject)
					results = append(results, result.GetID())
				},
			)

			nextToken := paginator.Page()
			require.Equal(t, tc.expected, results)
			require.Equal(t, tc.expectedNextToken, nextToken)
		})
	}

}

// helpers for pagination tests

// implements memdb.ResultIterator interface
type testResultIterator struct {
	results chan interface{}
	idx     int
}

func (i testResultIterator) Next() interface{} {
	select {
	case result := <-i.results:
		return result
	default:
		return nil
	}
}

// not used, but required to implement memdb.ResultIterator
func (i testResultIterator) WatchCh() <-chan struct{} {
	return make(<-chan struct{})
}

type mockObject struct {
	id string
}

func (m *mockObject) GetID() string {
	return m.id
}

func newTestIterator(ids []string) memdb.ResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, id := range ids {
		iter.results <- &mockObject{id: id}
	}
	return iter
}

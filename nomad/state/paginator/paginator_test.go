// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestPaginator(t *testing.T) {
	ci.Parallel(t)
	ids := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}

	cases := []struct {
		name              string
		perPage           int32
		nextToken         string
		tokenizer         Tokenizer[*mockObject]
		expected          []string
		expectedNextToken string
		expectedError     string
	}{
		{
			name:              "size-3 page-1",
			perPage:           3,
			tokenizer:         IDTokenizer[*mockObject](""),
			expected:          []string{"0", "1", "2"},
			expectedNextToken: "3",
		},
		{
			name:              "size-5 page-2 stop before end",
			perPage:           5,
			tokenizer:         IDTokenizer[*mockObject]("3"),
			nextToken:         "3",
			expected:          []string{"3", "4", "5", "6", "7"},
			expectedNextToken: "8",
		},
		{
			name:              "page-2 reading off the end",
			perPage:           10,
			tokenizer:         IDTokenizer[*mockObject]("5"),
			nextToken:         "5",
			expected:          []string{"5", "6", "7", "8", "9", "10", "11"},
			expectedNextToken: "",
		},
		{
			name:    "when numbers are strings",
			perPage: 2,
			// lexicographically, "10" < "2"
			nextToken:         "10",
			tokenizer:         IDTokenizer[*mockObject]("10"),
			expected:          []string{"2", "3"},
			expectedNextToken: "4",
		},
		{
			name:    "when numbers are numbers",
			perPage: 2,
			// a bare "10" target exercises the legacy index-only path: "10" is
			// parsed as uint64(10) and compared numerically with the index.
			nextToken:         "10",
			tokenizer:         ModifyIndexAndNamespaceIDTokenizer[*mockObject]("10"),
			expected:          []string{"10", "11"},
			expectedNextToken: "",
		},
		{
			name:    "when the next token is a full cursor",
			perPage: 2,
			// an empty token starts from the beginning; the next token is the
			// full "<index>.<namespace>.<id>" cursor (namespace is empty here).
			nextToken:         "",
			tokenizer:         ModifyIndexAndNamespaceIDTokenizer[*mockObject](""),
			expected:          []string{"0", "1"},
			expectedNextToken: "2..2",
		},
		{
			name:              "starting off the end",
			perPage:           5,
			nextToken:         "a",
			tokenizer:         IDTokenizer[*mockObject]("a"),
			expected:          []string{},
			expectedNextToken: "",
		},
		{
			name:          "error during append",
			expectedError: "failed to append",
			tokenizer:     IDTokenizer[*mockObject](""),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			iter := newTestIterator(ids)
			opts := structs.QueryOptions{
				PerPage:   tc.perPage,
				NextToken: tc.nextToken,
			}

			paginator, err := NewPaginator(iter, opts, nil, tc.tokenizer,
				func(result *mockObject) (string, error) {
					if tc.expectedError != "" {
						return "", errors.New(tc.expectedError)
					}
					return result.id, nil
				},
			)
			must.NoError(t, err)

			results, nextToken, err := paginator.Page()
			if tc.expectedError == "" {
				must.NoError(t, err)
				must.Eq(t, tc.expected, results)
				must.Eq(t, tc.expectedNextToken, nextToken)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedError)
			}
		})
	}

	// The cases above can only vary the id, so their compound tokens always
	// have an empty namespace. This case drives a full
	// "<index>.<namespace>.<id>" cursor through the paginator when several
	// objects share an index, exercising the namespace/id tiebreaker in the
	// seek path rather than just in the tokenizer in isolation.
	t.Run("resumes from a full namespaced cursor", func(t *testing.T) {
		// Ordered as memdb iterates: by index, then namespace, then id.
		mocks := []*mockObject{
			{index: 4, namespace: "teamA", id: "old"},
			{index: 5, namespace: "teamA", id: "web"},
			{index: 5, namespace: "teamB", id: "api"},
			{index: 5, namespace: "teamB", id: "web"},
			{index: 6, namespace: "teamA", id: "new"},
		}

		iter := newTestIteratorWithMocks(mocks)
		opts := structs.QueryOptions{
			PerPage:   2,
			NextToken: "5.teamB.api",
		}

		paginator, err := NewPaginator(iter, opts, nil,
			ModifyIndexAndNamespaceIDTokenizer[*mockObject]("5.teamB.api"),
			func(result *mockObject) (string, error) {
				return result.id, nil
			},
		)
		must.NoError(t, err)

		results, nextToken, err := paginator.Page()
		must.NoError(t, err)
		// Skips 4.teamA.old (lower index) and 5.teamA.web (same index, earlier
		// namespace) and resumes exactly at 5.teamB.api.
		must.Eq(t, []string{"api", "web"}, results)
		// The next page's cursor is a full three-part token.
		must.Eq(t, "6.teamA.new", nextToken)
	})
}

// helpers for pagination tests

// implements Iterator interface
type testResultIterator struct {
	results chan interface{}
}

func (i testResultIterator) Next() interface{} {
	select {
	case raw := <-i.results:
		if raw == nil {
			return nil
		}

		m := raw.(*mockObject)
		return m
	default:
		return nil
	}
}

type mockObject struct {
	index     uint64
	id        string
	namespace string
}

func (m *mockObject) GetNamespace() string {
	return m.namespace
}

func (m *mockObject) GetModifyIndex() uint64 {
	return m.index
}

func (m *mockObject) GetID() string {
	return m.id
}

func newTestIterator(ids []string) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for x, id := range ids {
		iter.results <- &mockObject{
			index: uint64(x),
			id:    id,
		}
	}
	return iter
}

func newTestIteratorWithMocks(mocks []*mockObject) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, m := range mocks {
		iter.results <- m
	}
	return iter
}

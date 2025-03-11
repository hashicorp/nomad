// Copyright (c) HashiCorp, Inc.
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
			// "10" is converted to uint64(10) and compared with uint64 index
			nextToken:         "10",
			tokenizer:         ModifyIndexTokenizer[*mockObject]("10"),
			expected:          []string{"10", "11"},
			expectedNextToken: "",
		},
		{
			name:    "when zero is a number",
			perPage: 2,
			// "" is converted to uint64(0) and compared with uint64 index
			nextToken:         "",
			tokenizer:         ModifyIndexTokenizer[*mockObject](""),
			expected:          []string{"0", "1"},
			expectedNextToken: "2",
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

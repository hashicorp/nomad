// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestPaginator(t *testing.T) {
	ci.Parallel(t)
	ids := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	cases := []struct {
		name              string
		perPage           int32
		nextToken         string
		expected          []string
		expectedNextToken string
		expectedError     string
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
		{
			name:          "error during append",
			expectedError: "failed to append",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			iter := newTestIterator(ids)
			tokenizer := testTokenizer{}
			opts := structs.QueryOptions{
				PerPage:   tc.perPage,
				NextToken: tc.nextToken,
			}

			results := []string{}
			paginator, err := NewPaginator(iter, tokenizer, nil, opts,
				func(raw interface{}) error {
					if tc.expectedError != "" {
						return errors.New(tc.expectedError)
					}

					result := raw.(*mockObject)
					results = append(results, result.id)
					return nil
				},
			)
			require.NoError(t, err)

			nextToken, err := paginator.Page()
			if tc.expectedError == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expected, results)
				require.Equal(t, tc.expectedNextToken, nextToken)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
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
	id        string
	namespace string
}

func (m *mockObject) GetNamespace() string {
	return m.namespace
}

func newTestIterator(ids []string) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, id := range ids {
		iter.results <- &mockObject{id: id}
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

// implements Tokenizer interface
type testTokenizer struct{}

func (t testTokenizer) GetToken(raw interface{}) string {
	return raw.(*mockObject).id
}

package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

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
			results := []string{}

			paginator, err := NewPaginator(iter,
				structs.QueryOptions{
					PerPage:   tc.perPage,
					NextToken: tc.nextToken,
					Ascending: true,
				},
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

// implements memdb.ResultIterator interface
type testResultIterator struct {
	results chan interface{}
}

func (i testResultIterator) Next() (string, interface{}) {
	select {
	case raw := <-i.results:
		if raw == nil {
			return "", nil
		}

		m := raw.(*mockObject)
		return m.id, m
	default:
		return "", nil
	}
}

type mockObject struct {
	id string
}

func newTestIterator(ids []string) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, id := range ids {
		iter.results <- &mockObject{id: id}
	}
	return iter
}

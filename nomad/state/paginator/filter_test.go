package paginator

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestGenericFilter(t *testing.T) {
	ci.Parallel(t)
	ids := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	filters := []Filter{GenericFilter{
		Allow: func(raw interface{}) (bool, error) {
			result := raw.(*mockObject)
			return result.id > "5", nil
		},
	}}
	iter := newTestIterator(ids)
	tokenizer := testTokenizer{}
	opts := structs.QueryOptions{
		PerPage: 3,
	}
	results := []string{}
	paginator, err := NewPaginator(iter, tokenizer, filters, opts,
		func(raw interface{}) error {
			result := raw.(*mockObject)
			results = append(results, result.id)
			return nil
		},
	)
	require.NoError(t, err)

	nextToken, err := paginator.Page()
	require.NoError(t, err)

	expected := []string{"6", "7", "8"}
	require.Equal(t, "9", nextToken)
	require.Equal(t, expected, results)
}

func TestNamespaceFilter(t *testing.T) {
	ci.Parallel(t)

	mocks := []*mockObject{
		{namespace: "default"},
		{namespace: "dev"},
		{namespace: "qa"},
		{namespace: "region-1"},
	}

	cases := []struct {
		name      string
		allowable map[string]bool
		expected  []string
	}{
		{
			name:     "nil map",
			expected: []string{"default", "dev", "qa", "region-1"},
		},
		{
			name:      "allow default",
			allowable: map[string]bool{"default": true},
			expected:  []string{"default"},
		},
		{
			name:      "allow multiple",
			allowable: map[string]bool{"default": true, "dev": false, "qa": true},
			expected:  []string{"default", "qa"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filters := []Filter{NamespaceFilter{
				AllowableNamespaces: tc.allowable,
			}}
			iter := newTestIteratorWithMocks(mocks)
			tokenizer := testTokenizer{}
			opts := structs.QueryOptions{
				PerPage: int32(len(mocks)),
			}

			results := []string{}
			paginator, err := NewPaginator(iter, tokenizer, filters, opts,
				func(raw interface{}) error {
					result := raw.(*mockObject)
					results = append(results, result.namespace)
					return nil
				},
			)
			require.NoError(t, err)

			nextToken, err := paginator.Page()
			require.NoError(t, err)
			require.Equal(t, "", nextToken)
			require.Equal(t, tc.expected, results)
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"testing"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
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

func BenchmarkEvalListFilter(b *testing.B) {
	const evalCount = 100_000

	b.Run("filter with index", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.EvalsByNamespace(nil, structs.DefaultNamespace)
			var lastSeen string
			var countSeen int
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				eval := raw.(*structs.Evaluation)
				lastSeen = eval.ID
				countSeen++
			}
			if countSeen < evalCount/2 {
				b.Fatalf("failed: %d evals seen, lastSeen=%s", countSeen, lastSeen)
			}
		}
	})

	b.Run("filter with go-bexpr", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)
		evaluator, err := bexpr.CreateEvaluator(`Namespace == "default"`)
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.Evals(nil, false)
			var lastSeen string
			var countSeen int
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				match, err := evaluator.Evaluate(raw)
				if !match || err != nil {
					continue
				}
				eval := raw.(*structs.Evaluation)
				lastSeen = eval.ID
				countSeen++
			}
			if countSeen < evalCount/2 {
				b.Fatalf("failed: %d evals seen, lastSeen=%s", countSeen, lastSeen)
			}
		}
	})

	b.Run("paginated filter with index", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)
		opts := structs.QueryOptions{
			PerPage: 100,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.EvalsByNamespace(nil, structs.DefaultNamespace)
			tokenizer := NewStructsTokenizer(iter, StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
				eval := raw.(*structs.Evaluation)
				evals = append(evals, eval)
				return nil
			})
			if err != nil {
				b.Fatalf("failed: %v", err)
			}
			paginator.Page()
		}
	})

	b.Run("paginated filter with go-bexpr", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)
		opts := structs.QueryOptions{
			PerPage: 100,
			Filter:  `Namespace == "default"`,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.Evals(nil, false)
			tokenizer := NewStructsTokenizer(iter, StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
				eval := raw.(*structs.Evaluation)
				evals = append(evals, eval)
				return nil
			})
			if err != nil {
				b.Fatalf("failed: %v", err)
			}
			paginator.Page()
		}
	})

	b.Run("paginated filter with index last page", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)

		// Find the last eval ID.
		iter, _ := state.Evals(nil, false)
		var lastSeen string
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			eval := raw.(*structs.Evaluation)
			lastSeen = eval.ID
		}

		opts := structs.QueryOptions{
			PerPage:   100,
			NextToken: lastSeen,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.EvalsByNamespace(nil, structs.DefaultNamespace)
			tokenizer := NewStructsTokenizer(iter, StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
				eval := raw.(*structs.Evaluation)
				evals = append(evals, eval)
				return nil
			})
			if err != nil {
				b.Fatalf("failed: %v", err)
			}
			paginator.Page()
		}
	})

	b.Run("paginated filter with go-bexpr last page", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount)

		// Find the last eval ID.
		iter, _ := state.Evals(nil, false)
		var lastSeen string
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			eval := raw.(*structs.Evaluation)
			lastSeen = eval.ID
		}

		opts := structs.QueryOptions{
			PerPage:   100,
			NextToken: lastSeen,
			Filter:    `Namespace == "default"`,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := state.Evals(nil, false)
			tokenizer := NewStructsTokenizer(iter, StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
				eval := raw.(*structs.Evaluation)
				evals = append(evals, eval)
				return nil
			})
			if err != nil {
				b.Fatalf("failed: %v", err)
			}
			paginator.Page()
		}
	})
}

// -----------------
// BENCHMARK HELPER FUNCTIONS

func setupPopulatedState(b *testing.B, evalCount int) *state.StateStore {
	evals := generateEvals(evalCount)

	index := uint64(0)
	var err error
	state := state.TestStateStore(b)
	for _, eval := range evals {
		index++
		err = state.UpsertEvals(
			structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
	}
	if err != nil {
		b.Fatalf("failed: %v", err)
	}
	return state
}

func generateEvals(count int) []*structs.Evaluation {
	evals := []*structs.Evaluation{}
	ns := structs.DefaultNamespace
	for i := 0; i < count; i++ {
		if i > count/2 {
			ns = "other"
		}
		evals = append(evals, generateEval(i, ns))
	}
	return evals
}

func generateEval(i int, ns string) *structs.Evaluation {
	now := time.Now().UTC().UnixNano()
	return &structs.Evaluation{
		ID:         uuid.Generate(),
		Namespace:  ns,
		Priority:   50,
		Type:       structs.JobTypeService,
		JobID:      uuid.Generate(),
		Status:     structs.EvalStatusPending,
		CreateTime: now,
		ModifyTime: now,
	}
}

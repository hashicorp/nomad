package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

func BenchmarkEvalListFilter(b *testing.B) {
	const evalCount = 100_000

	b.Run("filter with index", func(b *testing.B) {
		store := setupPopulatedState(b, evalCount)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := store.EvalsByNamespace(nil, structs.DefaultNamespace)
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
		store := setupPopulatedState(b, evalCount)
		evaluator, err := bexpr.CreateEvaluator(`Namespace == "default"`)
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := store.Evals(nil, false)
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
		store := setupPopulatedState(b, evalCount)
		opts := structs.QueryOptions{
			PerPage: 100,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := store.EvalsByNamespace(nil, structs.DefaultNamespace)
			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
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
		store := setupPopulatedState(b, evalCount)
		opts := structs.QueryOptions{
			PerPage: 100,
			Filter:  `Namespace == "default"`,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			iter, _ := store.Evals(nil, false)
			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
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
		store := setupPopulatedState(b, evalCount)

		// Find the last eval ID.
		iter, _ := store.Evals(nil, false)
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
			iter, _ := store.EvalsByNamespace(nil, structs.DefaultNamespace)
			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
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
		store := setupPopulatedState(b, evalCount)

		// Find the last eval ID.
		iter, _ := store.Evals(nil, false)
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
			iter, _ := store.Evals(nil, false)
			tokenizer := paginator.NewStructsTokenizer(iter, paginator.StructsTokenizerOptions{WithID: true})

			var evals []*structs.Evaluation
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, opts, func(raw interface{}) error {
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

func setupPopulatedState(b *testing.B, evalCount int) *StateStore {
	evals := generateEvals(evalCount)

	index := uint64(0)
	var err error
	store := TestStateStore(b)
	for _, eval := range evals {
		index++
		err = store.UpsertEvals(
			structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
	}
	if err != nil {
		b.Fatalf("failed: %v", err)
	}
	return store
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

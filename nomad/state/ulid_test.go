package state

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	ulid "github.com/oklog/ulid/v2"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func Benchmark_SortableIDs_Iteration(b *testing.B) {

	const evalCount = 10000

	b.Run("uuid iterate", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount, uuidGenerator())
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
			if countSeen < evalCount {
				b.Fatalf("failed: %d evals seen, lastSeen=%s", countSeen, lastSeen)
			}
		}
	})

	b.Run("ulid iterate inserted closely", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		state := setupPopulatedState(b, evalCount,
			ulidGenerator(entropy))
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
			if countSeen < evalCount {
				b.Fatalf("failed: %d evals seen, lastSeen=%s", countSeen, lastSeen)
			}
		}
	})

	b.Run("ulid iterate inserted over hours", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		state := setupPopulatedState(b, evalCount,
			ulidGeneratorForPast(entropy, 4, evalCount))
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
			if countSeen < evalCount {
				b.Fatalf("failed: %d evals seen, lastSeen=%s", countSeen, lastSeen)
			}
		}
	})

}

func Benchmark_SortableIDs_Upserts(b *testing.B) {

	const evalCount = 10000

	b.Run("uuid upsert eval", func(b *testing.B) {
		state := setupPopulatedState(b, evalCount, uuidGenerator())
		evals := generateEvals(evalCount, uuidGenerator())

		index := uint64(0)
		var err error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			snap, _ := state.Snapshot()
			for _, eval := range evals {
				index++
				err = snap.UpsertEvals(
					structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
			}
		}
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
	})

	b.Run("ulid upsert eval closely", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		state := setupPopulatedState(b, evalCount,
			ulidGeneratorForPast(entropy, 4, evalCount))
		evals := generateEvals(evalCount, ulidGenerator(entropy))

		index := uint64(0)
		var err error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			snap, _ := state.Snapshot()
			for _, eval := range evals {
				index++
				err = snap.UpsertEvals(
					structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
			}
		}
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
	})

	b.Run("ulid upsert eval over hours", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		state := setupPopulatedState(b, evalCount,
			ulidGeneratorForPast(entropy, 4, evalCount))
		evals := generateEvals(evalCount, ulidGeneratorForPast(entropy, 4, evalCount))

		index := uint64(0)
		var err error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			snap, _ := state.Snapshot()
			for _, eval := range evals {
				index++
				err = snap.UpsertEvals(
					structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
			}
		}
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
	})

}

// -----------------
// BENCHMARK HELPER FUNCTIONS

func setupPopulatedState(b *testing.B, evalCount int, idFn func(int) string) *StateStore {
	evals := generateEvals(evalCount, idFn)

	index := uint64(0)
	var err error
	state := TestStateStore(b)
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

func generateEvals(count int, idFn func(int) string) []*structs.Evaluation {
	evals := []*structs.Evaluation{}
	for i := 0; i < count; i++ {
		now := time.Now().UTC().UnixNano()
		eval := &structs.Evaluation{
			ID:         idFn(i),
			Namespace:  structs.DefaultNamespace,
			Priority:   50,
			Type:       structs.JobTypeService,
			JobID:      uuid.Generate(),
			Status:     structs.EvalStatusPending,
			CreateTime: now,
			ModifyTime: now,
		}
		evals = append(evals, eval)

	}
	return evals
}

func ulidGenerator(entropy *ulid.MonotonicEntropy) func(int) string {
	return func(_ int) string {
		return generateULIDAt(entropy, ulid.Timestamp(time.Now()))
	}
}

// generates ULIDs so that they're spread evenly out over the past several hours
func ulidGeneratorForPast(entropy *ulid.MonotonicEntropy, hoursPast int, totalCount int) func(int) string {
	base := time.Now().Add(time.Duration(-hoursPast) * time.Hour)
	multiplier := uint64(hoursPast * totalCount)
	return func(idx int) string {
		ts := ulid.Timestamp(base) + uint64(idx)*multiplier
		return generateULIDAt(entropy, ts)
	}
}

func generateULIDAt(entropy *ulid.MonotonicEntropy, ts uint64) string {
	id := ulid.MustNew(ts, entropy)
	buf, _ := id.MarshalBinary()
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func uuidGenerator() func(int) string {
	return func(_ int) string {
		return uuid.Generate()
	}
}

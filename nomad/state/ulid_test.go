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

func generateEvals(count int, idFn func() string) []*structs.Evaluation {
	evals := []*structs.Evaluation{}
	for i := 0; i < count; i++ {
		now := time.Now().UTC().UnixNano()
		eval := &structs.Evaluation{
			ID:         idFn(),
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

func generateULID(entropy *ulid.MonotonicEntropy) string {

	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	buf, _ := id.MarshalBinary()
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func Benchmark_EvalsIDGeneration(b *testing.B) {

	const evalCount = 10000

	b.Run("uuid insert", func(b *testing.B) {
		evals := generateEvals(evalCount, uuid.Generate)

		index := uint64(0)
		var err error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			state := TestStateStore(b)
			for _, eval := range evals {
				index++
				err = state.UpsertEvals(
					structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
			}
		}
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
	})

	b.Run("uuid iterate", func(b *testing.B) {
		evals := generateEvals(evalCount, uuid.Generate)

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

	b.Run("ulid insert", func(b *testing.B) {

		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		evals := generateEvals(evalCount, func() string {
			return generateULID(entropy)
		})

		index := uint64(0)
		var err error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			state := TestStateStore(b)
			for _, eval := range evals {
				index++
				err = state.UpsertEvals(
					structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
			}
		}
		if err != nil {
			b.Fatalf("failed: %v", err)
		}
	})

	b.Run("ulid iterate", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
		evals := generateEvals(evalCount, func() string {
			return generateULID(entropy)
		})

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

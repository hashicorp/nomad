// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ---------------------------------------------------------------------------
// Setup helpers
// ---------------------------------------------------------------------------

// benchEntry pairs a backend name with an open StateDB ready for benchmarking.
type benchEntry struct {
	name string
	db   StateDB
}

// benchDBs returns one BoltDB and one SQLite StateDB, both using a null logger
// so that log I/O does not skew timing results. Cleanup is registered on b.
func benchDBs(b *testing.B) []benchEntry {
	b.Helper()
	logger := hclog.NewNullLogger()

	boltDir := b.TempDir()
	boltDB, err := NewBoltStateDB(logger, boltDir)
	if err != nil {
		b.Fatalf("create boltdb: %v", err)
	}
	b.Cleanup(func() { _ = boltDB.Close() })

	sqliteDir := b.TempDir()
	sqliteDB, err := NewSQLiteStateDB(logger, sqliteDir)
	if err != nil {
		b.Fatalf("create sqlite: %v", err)
	}
	b.Cleanup(func() { _ = sqliteDB.Close() })

	return []benchEntry{
		{"boltdb", boltDB},
		{"sqlite", sqliteDB},
	}
}

// populate inserts n mock allocations into db and returns them. The benchmark
// timer is stopped during population so that setup time is not counted.
func populate(b *testing.B, db StateDB, n int) []*structs.Allocation {
	b.Helper()
	b.StopTimer()
	allocs := make([]*structs.Allocation, n)
	for i := range allocs {
		allocs[i] = mock.Alloc()
		if err := db.PutAllocation(allocs[i]); err != nil {
			b.Fatalf("populate: put alloc: %v", err)
		}
	}
	b.StartTimer()
	return allocs
}

// checkResult builds a realistic CheckQueryResult for benchmarking.
func checkResult(allocID string) *structs.CheckQueryResult {
	return &structs.CheckQueryResult{
		ID:        structs.CheckID(allocID + "-chk"),
		Mode:      "healthiness",
		Status:    "passing",
		Output:    "nomad: tcp ok",
		Timestamp: 1,
		Group:     "group",
		Task:      "task",
		Service:   "service",
		Check:     "check",
	}
}

// ---------------------------------------------------------------------------
// PutAllocation
// ---------------------------------------------------------------------------

// BenchmarkStateDB_PutAllocation measures the cost of persisting a single
// allocation sequentially. This is representative of the steady-state path
// where one alloc at a time receives an update.
func BenchmarkStateDB_PutAllocation(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			alloc := mock.Alloc()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := entry.db.PutAllocation(alloc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateDB_PutAllocation_Parallel measures write throughput when
// multiple goroutines each write a distinct allocation concurrently. Unlike the
// batch benchmark below this does not use WithBatchMode, so every write is its
// own transaction for both backends.
func BenchmarkStateDB_PutAllocation_Parallel(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				// Each goroutine works on its own allocation so we are not
				// serialising on a single hot key.
				alloc := mock.Alloc()
				for pb.Next() {
					if err := entry.db.PutAllocation(alloc); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkStateDB_PutAllocation_Batch measures concurrent writes issued with
// WithBatchMode. BoltDB can coalesce these into fewer transactions; SQLite
// treats the option as a no-op and serialises normally through its single
// connection.
func BenchmarkStateDB_PutAllocation_Batch(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				alloc := mock.Alloc()
				for pb.Next() {
					if err := entry.db.PutAllocation(alloc, WithBatchMode()); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// GetAllAllocations
// ---------------------------------------------------------------------------

// BenchmarkStateDB_GetAllAllocations measures how long it takes to read back
// all persisted allocations. The database is pre-populated with n allocations
// before the timer starts; only the read is timed. This is the critical path
// on agent restart.
func BenchmarkStateDB_GetAllAllocations(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for _, entry := range benchDBs(b) {
				b.Run(entry.name, func(b *testing.B) {
					b.ReportAllocs()
					populate(b, entry.db, n)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						allocs, _, err := entry.db.GetAllAllocations()
						if err != nil {
							b.Fatal(err)
						}
						if len(allocs) != n {
							b.Fatalf("got %d allocs, want %d", len(allocs), n)
						}
					}
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task state
// ---------------------------------------------------------------------------

// BenchmarkStateDB_PutTaskState measures writing a TaskState for a single
// task, which happens frequently during a task's lifecycle.
func BenchmarkStateDB_PutTaskState(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			state := structs.NewTaskState()
			state.State = structs.TaskStateRunning
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := entry.db.PutTaskState("alloc-id", "web", state); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateDB_GetTaskRunnerState measures reading back both the
// LocalState and TaskState for a single task.
func BenchmarkStateDB_GetTaskRunnerState(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()

			// Pre-populate both halves of the task state.
			b.StopTimer()
			state := structs.NewTaskState()
			state.State = structs.TaskStateRunning
			if err := entry.db.PutTaskState("alloc-id", "web", state); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := entry.db.GetTaskRunnerState("alloc-id", "web")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Check results
// ---------------------------------------------------------------------------

// BenchmarkStateDB_PutCheckResult measures writing a single check result,
// which happens on every health-check tick.
func BenchmarkStateDB_PutCheckResult(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			qr := checkResult("alloc-bench")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := entry.db.PutCheckResult("alloc-bench", qr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateDB_GetCheckResults measures scanning all check results.
// n is the number of distinct (allocID, checkID) pairs pre-loaded.
func BenchmarkStateDB_GetCheckResults(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for _, entry := range benchDBs(b) {
				b.Run(entry.name, func(b *testing.B) {
					b.ReportAllocs()

					b.StopTimer()
					for i := 0; i < n; i++ {
						allocID := fmt.Sprintf("alloc-%d", i)
						qr := checkResult(allocID)
						if err := entry.db.PutCheckResult(allocID, qr); err != nil {
							b.Fatal(err)
						}
					}
					b.StartTimer()

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						results, err := entry.db.GetCheckResults()
						if err != nil {
							b.Fatal(err)
						}
						if len(results) != n {
							b.Fatalf("got %d check results, want %d", len(results), n)
						}
					}
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteAllocationBucket
// ---------------------------------------------------------------------------

// BenchmarkStateDB_DeleteAllocationBucket measures the time to remove one
// allocation and all its associated state. Because the allocation must exist
// before it can be deleted, setup (PutAllocation) is performed outside the
// timed section.
func BenchmarkStateDB_DeleteAllocationBucket(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Insert a fresh alloc, then time only the delete.
				alloc := mock.Alloc()
				b.StopTimer()
				if err := entry.db.PutAllocation(alloc); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()

				if err := entry.db.DeleteAllocationBucket(alloc.ID); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Realistic allocation lifecycle
// ---------------------------------------------------------------------------

// BenchmarkStateDB_AllocLifecycle exercises a condensed version of the write
// pattern a single allocation produces during its lifetime:
//
//  1. PutAllocation (scheduler places the alloc)
//  2. PutNetworkStatus (driver reports network info)
//  3. PutTaskState × 2 (task transitions: pending → running → dead)
//  4. PutCheckResult (health-check tick)
//  5. DeleteAllocationBucket (alloc completes)
//
// All five steps count toward b.N; one "lifecycle" equals one b.N unit.
func BenchmarkStateDB_AllocLifecycle(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			ns := mock.AllocNetworkStatus()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				alloc := mock.Alloc()
				id := alloc.ID

				if err := entry.db.PutAllocation(alloc); err != nil {
					b.Fatal(err)
				}

				if err := entry.db.PutNetworkStatus(id, ns); err != nil {
					b.Fatal(err)
				}

				running := structs.NewTaskState()
				running.State = structs.TaskStateRunning
				if err := entry.db.PutTaskState(id, "web", running); err != nil {
					b.Fatal(err)
				}

				dead := structs.NewTaskState()
				dead.State = structs.TaskStateDead
				if err := entry.db.PutTaskState(id, "web", dead); err != nil {
					b.Fatal(err)
				}

				if err := entry.db.PutCheckResult(id, checkResult(id)); err != nil {
					b.Fatal(err)
				}

				if err := entry.db.DeleteAllocationBucket(id); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateDB_AllocLifecycle_Parallel runs the same lifecycle as above
// but with GOMAXPROCS goroutines concurrently, each operating on its own
// allocation. This reflects the real agent workload where many allocs progress
// through their lifecycle simultaneously.
func BenchmarkStateDB_AllocLifecycle_Parallel(b *testing.B) {
	for _, entry := range benchDBs(b) {
		b.Run(entry.name, func(b *testing.B) {
			b.ReportAllocs()
			ns := mock.AllocNetworkStatus()
			running := structs.NewTaskState()
			running.State = structs.TaskStateRunning
			dead := structs.NewTaskState()
			dead.State = structs.TaskStateDead

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					alloc := mock.Alloc()
					id := alloc.ID

					if err := entry.db.PutAllocation(alloc); err != nil {
						b.Fatal(err)
					}
					if err := entry.db.PutNetworkStatus(id, ns); err != nil {
						b.Fatal(err)
					}
					if err := entry.db.PutTaskState(id, "web", running); err != nil {
						b.Fatal(err)
					}
					if err := entry.db.PutTaskState(id, "web", dead); err != nil {
						b.Fatal(err)
					}
					if err := entry.db.PutCheckResult(id, checkResult(id)); err != nil {
						b.Fatal(err)
					}
					if err := entry.db.DeleteAllocationBucket(id); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

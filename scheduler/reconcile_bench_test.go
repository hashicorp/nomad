package scheduler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func BenchmarkReconciler(b *testing.B) {

	node := mock.Node()

	jobv3 := mock.Job()
	jobv3.Version = 3

	tg := jobv3.TaskGroups[0]
	tg.Update = structs.DefaultUpdateStrategy
	tg.ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      time.Hour,
		Delay:         0,
		DelayFunction: "constant",
		MaxDelay:      time.Hour,
		Unlimited:     false,
	}
	tgName := tg.Name

	jobv2 := mock.Job()
	jobv2.Version = 2

	d := structs.NewDeployment(jobv3, 50)

	allocs := []*structs.Allocation{
		{
			ID:            "alloc-0-should-keep",
			Job:           jobv3,
			JobID:         jobv3.ID,
			Namespace:     structs.DefaultNamespace,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusRunning,
			TaskGroup:     tgName,
			Name:          tgName + "[0]",
			NodeID:        node.ID,
		},
		{
			ID:            "alloc-2-should-replace",
			Job:           jobv3,
			JobID:         jobv3.ID,
			Namespace:     structs.DefaultNamespace,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusFailed,
			TaskGroup:     tgName,
			Name:          tgName + "[2]",
			NodeID:        node.ID,
		},
		{
			ID:            "alloc-1-should-keep",
			Job:           jobv3,
			JobID:         jobv3.ID,
			Namespace:     structs.DefaultNamespace,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusRunning,
			TaskGroup:     tgName,
			Name:          tgName + "[1]",
			NodeID:        node.ID,
		},
		{
			ID:            "alloc-4-should-stop",
			Job:           jobv2,
			JobID:         jobv2.ID,
			Namespace:     structs.DefaultNamespace,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusRunning,
			TaskGroup:     tgName,
			Name:          tgName + "[4]",
			NodeID:        node.ID,
		},
		{
			ID:            "alloc-3-should-destructive-update",
			Job:           jobv2,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusRunning,
			TaskGroup:     tgName,
			Name:          tgName + "[3]",
			NodeID:        node.ID,
		},
	}

	mAlloc := mock.Alloc()
	for _, alloc := range allocs {
		alloc.AllocatedResources = mAlloc.AllocatedResources
		alloc.RescheduleTracker = &structs.RescheduleTracker{
			Events: []*structs.RescheduleEvent{},
		}
	}

	eval := &structs.Evaluation{
		ID:    uuid.Generate(),
		JobID: jobv3.ID,
	}

	b.Run("existing implementation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			reconciler := NewAllocReconciler(testlog.HCLogger(b),
				allocUpdateFnInplace,
				false, jobv3.ID, jobv3, d, allocs,
				map[string]*structs.Node{}, uuid.Generate(),
				50, true)
			b.StartTimer()

			reconciler.Compute()
		}
	})

	b.Run("revisited implementation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reconcileServiceDeployment(eval, jobv3, tg, d, allocs, []*structs.Node{node})
		}
	})

}

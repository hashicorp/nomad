// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func LifecycleSideTask(resources structs.Resources, i int) *structs.Task {
	return &structs.Task{
		Name:   fmt.Sprintf("side-%d", i),
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
		},
		Lifecycle: &structs.TaskLifecycleConfig{
			Hook:    structs.TaskLifecycleHookPrestart,
			Sidecar: true,
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &resources,
	}
}

func LifecycleInitTask(resources structs.Resources, i int) *structs.Task {
	return &structs.Task{
		Name:   fmt.Sprintf("init-%d", i),
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
		},
		Lifecycle: &structs.TaskLifecycleConfig{
			Hook:    structs.TaskLifecycleHookPrestart,
			Sidecar: false,
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &resources,
	}
}

func LifecycleMainTask(resources structs.Resources, i int) *structs.Task {
	return &structs.Task{
		Name:   fmt.Sprintf("main-%d", i),
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &resources,
	}
}

type LifecycleTaskDef struct {
	Name      string
	RunFor    string
	ExitCode  int
	Hook      string
	IsSidecar bool
}

// LifecycleAllocFromTasks generates an Allocation with mock tasks that have
// the provided lifecycles.
func LifecycleAllocFromTasks(tasks []LifecycleTaskDef) *structs.Allocation {
	alloc := LifecycleAlloc()
	alloc.Job.TaskGroups[0].Tasks = []*structs.Task{}
	for _, task := range tasks {
		var lc *structs.TaskLifecycleConfig
		if task.Hook != "" {
			// TODO: task coordinator doesn't treat nil and empty structs the same
			lc = &structs.TaskLifecycleConfig{
				Hook:    task.Hook,
				Sidecar: task.IsSidecar,
			}
		}

		alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks,
			&structs.Task{
				Name:   task.Name,
				Driver: "mock_driver",
				Config: map[string]interface{}{
					"run_for":   task.RunFor,
					"exit_code": task.ExitCode},
				Lifecycle: lc,
				LogConfig: structs.DefaultLogConfig(),
				Resources: &structs.Resources{CPU: 100, MemoryMB: 256},
			},
		)
		alloc.TaskResources[task.Name] = &structs.Resources{CPU: 100, MemoryMB: 256}
		alloc.AllocatedResources.Tasks[task.Name] = &structs.AllocatedTaskResources{
			Cpu:    structs.AllocatedCpuResources{CpuShares: 100},
			Memory: structs.AllocatedMemoryResources{MemoryMB: 256},
		}
	}
	return alloc
}

func LifecycleAlloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"init": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"side": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"poststart": {
				CPU:      1000,
				MemoryMB: 256,
			},
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"init": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"side": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"poststart": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
			},
		},
		Job:           LifecycleJob(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func LifecycleJobWithPoststopDeploy() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeBatch,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:    "web",
				Count:   1,
				Migrate: structs.DefaultMigrateStrategy(),
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 0,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeFail,
				},
				Tasks: []*structs.Task{
					{
						Name:   "web",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "side",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: true,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "post",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook: structs.TaskLifecycleHookPoststop,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "init",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: false,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func LifecycleJobWithPoststartDeploy() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeBatch,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:    "web",
				Count:   1,
				Migrate: structs.DefaultMigrateStrategy(),
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 0,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeFail,
				},
				Tasks: []*structs.Task{
					{
						Name:   "web",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "side",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: true,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "post",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook: structs.TaskLifecycleHookPoststart,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "init",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: false,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func LifecycleAllocWithPoststopDeploy() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"init": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"side": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"post": {
				CPU:      1000,
				MemoryMB: 256,
			},
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"init": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"side": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"post": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
			},
		},
		Job:           LifecycleJobWithPoststopDeploy(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func LifecycleAllocWithPoststartDeploy() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789xyz",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"init": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"side": {
				CPU:      1000,
				MemoryMB: 256,
			},
			"post": {
				CPU:      1000,
				MemoryMB: 256,
			},
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"init": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"side": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
				"post": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
			},
		},
		Job:           LifecycleJobWithPoststartDeploy(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func VariableLifecycleJob(resources structs.Resources, main int, init int, side int) *structs.Job {
	var tasks []*structs.Task
	for i := 0; i < main; i++ {
		tasks = append(tasks, LifecycleMainTask(resources, i))
	}
	for i := 0; i < init; i++ {
		tasks = append(tasks, LifecycleInitTask(resources, i))
	}
	for i := 0; i < side; i++ {
		tasks = append(tasks, LifecycleSideTask(resources, i))
	}
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "web",
				Count: 1,
				Tasks: tasks,
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func LifecycleJob() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeBatch,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "web",
				Count: 1,
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 0,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeFail,
				},
				Tasks: []*structs.Task{
					{
						Name:   "web",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "side",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: true,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "init",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPrestart,
							Sidecar: false,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
					{
						Name:   "poststart",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "1s",
						},
						Lifecycle: &structs.TaskLifecycleConfig{
							Hook:    structs.TaskLifecycleHookPoststart,
							Sidecar: false,
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      1000,
							MemoryMB: 256,
						},
					},
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

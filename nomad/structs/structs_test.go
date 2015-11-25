package structs

import (
	"github.com/hashicorp/go-multierror"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestJob_Validate(t *testing.T) {
	j := &Job{}
	err := j.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "job region") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "job ID") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "job name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[3].Error(), "job type") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[4].Error(), "priority") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[5].Error(), "datacenters") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[6].Error(), "task groups") {
		t.Fatalf("err: %s", err)
	}

	j = &Job{
		Region:      "global",
		ID:          GenerateUUID(),
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		Datacenters: []string{"dc1"},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			&TaskGroup{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			&TaskGroup{
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
		},
	}
	err = j.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "2 redefines 'web' from group 1") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "group 3 missing name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Task group 1 validation failed") {
		t.Fatalf("err: %s", err)
	}
}

func TestTaskGroup_Validate(t *testing.T) {
	tg := &TaskGroup{
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
		},
	}
	err := tg.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "group name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "count must be positive") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Missing tasks") {
		t.Fatalf("err: %s", err)
	}

	tg = &TaskGroup{
		Name:  "web",
		Count: 1,
		Tasks: []*Task{
			&Task{Name: "web"},
			&Task{Name: "web"},
			&Task{},
		},
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
		},
	}
	err = tg.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "2 redefines 'web' from task 1") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "Task 3 missing name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Task 1 validation failed") {
		t.Fatalf("err: %s", err)
	}
}

func TestTask_Validate(t *testing.T) {
	task := &Task{}
	err := task.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "task name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "task driver") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "task resources") {
		t.Fatalf("err: %s", err)
	}

	task = &Task{
		Name:      "web",
		Driver:    "docker",
		Resources: &Resources{},
	}
	err = task.Validate()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestConstraint_Validate(t *testing.T) {
	c := &Constraint{}
	err := c.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Missing constraint operand") {
		t.Fatalf("err: %s", err)
	}

	c = &Constraint{
		LTarget: "$attr.kernel.name",
		RTarget: "linux",
		Operand: "=",
	}
	err = c.Validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Perform additional regexp validation
	c.Operand = ConstraintRegex
	c.RTarget = "(foo"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "missing closing") {
		t.Fatalf("err: %s", err)
	}

	// Perform version validation
	c.Operand = ConstraintVersion
	c.RTarget = "~> foo"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Malformed constraint") {
		t.Fatalf("err: %s", err)
	}
}

func TestResource_NetIndex(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{Device: "eth0"},
			&NetworkResource{Device: "lo0"},
			&NetworkResource{Device: ""},
		},
	}
	if idx := r.NetIndex(&NetworkResource{Device: "eth0"}); idx != 0 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndex(&NetworkResource{Device: "lo0"}); idx != 1 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndex(&NetworkResource{Device: "eth1"}); idx != -1 {
		t.Fatalf("Bad: %d", idx)
	}
}

func TestResource_Superset(t *testing.T) {
	r1 := &Resources{
		CPU:      2000,
		MemoryMB: 2048,
		DiskMB:   10000,
		IOPS:     100,
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
	}

	if s, _ := r1.Superset(r1); !s {
		t.Fatalf("bad")
	}
	if s, _ := r1.Superset(r2); !s {
		t.Fatalf("bad")
	}
	if s, _ := r2.Superset(r1); s {
		t.Fatalf("bad")
	}
	if s, _ := r2.Superset(r2); !s {
		t.Fatalf("bad")
	}
}

func TestResource_Add(t *testing.T) {
	r1 := &Resources{
		CPU:      2000,
		MemoryMB: 2048,
		DiskMB:   10000,
		IOPS:     100,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         100,
				ReservedPorts: []Port{{"ssh", 22}},
			},
		},
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
		Networks: []*NetworkResource{
			&NetworkResource{
				IP:            "10.0.0.1",
				MBits:         50,
				ReservedPorts: []Port{{"web", 80}},
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		CPU:      3000,
		MemoryMB: 3072,
		DiskMB:   15000,
		IOPS:     150,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         150,
				ReservedPorts: []Port{{"ssh", 22}, {"web", 80}},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect, r1)
	}
}

func TestResource_Add_Network(t *testing.T) {
	r1 := &Resources{}
	r2 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        50,
				DynamicPorts: []Port{{"http", 0}, {"https", 0}},
			},
		},
	}
	r3 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        25,
				DynamicPorts: []Port{{"admin", 0}},
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	err = r1.Add(r3)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        75,
				DynamicPorts: []Port{{"http", 0}, {"https", 0}, {"admin", 0}},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect.Networks[0], r1.Networks[0])
	}
}

func TestEncodeDecode(t *testing.T) {
	type FooRequest struct {
		Foo string
		Bar int
		Baz bool
	}
	arg := &FooRequest{
		Foo: "test",
		Bar: 42,
		Baz: true,
	}
	buf, err := Encode(1, arg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out FooRequest
	err = Decode(buf[1:], &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(arg, &out) {
		t.Fatalf("bad: %#v %#v", arg, out)
	}
}

func TestInvalidServiceCheck(t *testing.T) {
	s := Service{
		Id:        "service-id",
		Name:      "service-name",
		PortLabel: "bar",
		Checks: []ServiceCheck{
			{

				Id:   "check-id",
				Name: "check-name",
				Type: "lol",
			},
		},
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("Service should be invalid")
	}
}

func TestDistinctCheckId(t *testing.T) {
	c1 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health",
		Interval: 2 * time.Second,
		Timeout:  3 * time.Second,
	}
	c2 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health1",
		Interval: 2 * time.Second,
		Timeout:  3 * time.Second,
	}

	c3 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health",
		Interval: 4 * time.Second,
		Timeout:  3 * time.Second,
	}

	if c1.Hash() == c2.Hash() || c1.Hash() == c3.Hash() || c3.Hash() == c2.Hash() {
		t.Fatalf("Checks need to be uniq c1: %s, c2: %s, c3: %s", c1.Hash(), c2.Hash(), c3.Hash())
	}

}

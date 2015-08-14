package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
)

func TestServiceStack_SetJob(t *testing.T) {
	_, ctx := testContext(t)
	stack := NewServiceStack(ctx, nil)

	job := mock.Job()
	stack.SetJob(job)

	if stack.binPack.priority != job.Priority {
		t.Fatalf("bad")
	}
	if !reflect.DeepEqual(stack.jobConstraint.constraints, job.Constraints) {
		t.Fatalf("bad")
	}
}

func TestServiceStack_Select_Size(t *testing.T) {
	// TODO
}

func TestServiceStack_Select_MetricsReset(t *testing.T) {
	// TODO
}

func TestServiceStack_Select_DriverFilter(t *testing.T) {
	// TODO
}

func TestServiceStack_Select_ConstraintFilter(t *testing.T) {
	// TODO
}

func TestServiceStack_Select_BinPack_Overflow(t *testing.T) {
	// TODO
}

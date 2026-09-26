package types

// package types is testdata for the generator

import "time"

//go:generate nomad-generate -log-level debug

// Multiregion is not top-level but comes before a top-level struct in the file
type Multiregion struct {
	Strategy *MultiregionStrategy
	Regions  []*MultiregionRegion
}

type MultiregionStrategy struct {
	MaxParallel int
	OnFailure   string
}

// Job is the top-level object
// nomad-generate: Copy,Diff
type Job struct {
	ID          string // ID is...
	CreateIndex uint64

	// MultiRegion is...
	Multiregion *Multiregion `foo:"what"` // MultiRegion could also be...

	Periodic           *PeriodicConfig
	Parameters         *ParameterizedJobConfig
	TaskGroupSummaries map[string]*TaskGroupSummary
	Affinities         []*Affinity
	Meta               map[string]string
	Payload            []byte
}

func (j *Job) SomeMethod() error { return nil }

func SomeFreeFunction() error { return nil }

type MultiregionRegion struct {
	Name        string
	Count       int
	Datacenters []string
	NodePool    string
	Meta        map[string]string
}

type Affinity struct {
	Name   string
	Counts []int
}

type PeriodicConfig struct {
	Enabled         bool
	Spec            string
	Specs           []string
	SpecType        string
	ProhibitOverlap bool
	TimeZone        string
	location        *time.Location
}

type ParameterizedJobConfig struct {
	Payload      string
	MetaRequired []string
	MetaOptional []string
}

type UpdateStrategy struct {
	Stagger          time.Duration
	MaxParallel      int
	HealthCheck      string
	MinHealthyTime   time.Duration
	HealthyDeadline  time.Duration
	ProgressDeadline time.Duration
	AutoRevert       bool
	AutoPromote      bool
	Canary           int
}

func (u *UpdateStrategy) Copy() *UpdateStrategy { return nil }

type TaskGroupSummary struct {
	Queued   int
	Complete int
	Failed   int
	Running  int
	Starting int
	Lost     int
	Unknown  int
}

// Service is disconnected from Job
// nomad-generate: Copy
type Service struct {
	Name     string
	TaskName string
	Connect  *Connect
	Tags     []string
	Checks   []*Check
	Meta     map[string]string
}

type Connect struct{}
type Check struct{}

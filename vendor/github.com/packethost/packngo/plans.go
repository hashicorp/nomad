package packngo

import (
	"fmt"
)

const planBasePath = "/plans"

// PlanService interface defines available plan methods
type PlanService interface {
	List(*ListOptions) ([]Plan, *Response, error)
}

type planRoot struct {
	Plans []Plan `json:"plans"`
}

// Plan represents a Packet service plan
type Plan struct {
	ID              string     `json:"id"`
	Slug            string     `json:"slug,omitempty"`
	Name            string     `json:"name,omitempty"`
	Description     string     `json:"description,omitempty"`
	Line            string     `json:"line,omitempty"`
	Specs           *Specs     `json:"specs,omitempty"`
	Pricing         *Pricing   `json:"pricing,omitempty"`
	DeploymentTypes []string   `json:"deployment_types"`
	Class           string     `json:"class"`
	AvailableIn     []Facility `json:"available_in"`
}

func (p Plan) String() string {
	return Stringify(p)
}

// Specs - the server specs for a plan
type Specs struct {
	Cpus     []*Cpus   `json:"cpus,omitempty"`
	Memory   *Memory   `json:"memory,omitempty"`
	Drives   []*Drives `json:"drives,omitempty"`
	Nics     []*Nics   `json:"nics,omitempty"`
	Features *Features `json:"features,omitempty"`
}

func (s Specs) String() string {
	return Stringify(s)
}

// Cpus - the CPU config details for specs on a plan
type Cpus struct {
	Count int    `json:"count,omitempty"`
	Type  string `json:"type,omitempty"`
}

func (c Cpus) String() string {
	return Stringify(c)
}

// Memory - the RAM config details for specs on a plan
type Memory struct {
	Total string `json:"total,omitempty"`
}

func (m Memory) String() string {
	return Stringify(m)
}

// Drives - the storage config details for specs on a plan
type Drives struct {
	Count int    `json:"count,omitempty"`
	Size  string `json:"size,omitempty"`
	Type  string `json:"type,omitempty"`
}

func (d Drives) String() string {
	return Stringify(d)
}

// Nics - the network hardware details for specs on a plan
type Nics struct {
	Count int    `json:"count,omitempty"`
	Type  string `json:"type,omitempty"`
}

func (n Nics) String() string {
	return Stringify(n)
}

// Features - other features in the specs for a plan
type Features struct {
	Raid bool `json:"raid,omitempty"`
	Txt  bool `json:"txt,omitempty"`
}

func (f Features) String() string {
	return Stringify(f)
}

// Pricing - the pricing options on a plan
type Pricing struct {
	Hour  float32 `json:"hour,omitempty"`
	Month float32 `json:"month,omitempty"`
}

func (p Pricing) String() string {
	return Stringify(p)
}

// PlanServiceOp implements PlanService
type PlanServiceOp struct {
	client *Client
}

// List method returns all available plans
func (s *PlanServiceOp) List(listOpt *ListOptions) ([]Plan, *Response, error) {
	root := new(planRoot)
	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s?%s", planBasePath, params)

	resp, err := s.client.DoRequest("GET", path, nil, root)
	if err != nil {
		return nil, resp, err
	}

	return root.Plans, resp, err
}

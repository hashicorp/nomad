package packngo

import (
	"fmt"
	"math"
)

const spotMarketRequestBasePath = "/spot-market-requests"

type SpotMarketRequestService interface {
	List(string, *ListOptions) ([]SpotMarketRequest, *Response, error)
	Create(*SpotMarketRequestCreateRequest, string) (*SpotMarketRequest, *Response, error)
	Delete(string, bool) (*Response, error)
	Get(string, *GetOptions) (*SpotMarketRequest, *Response, error)
}

type SpotMarketRequestCreateRequest struct {
	DevicesMax  int        `json:"devices_max"`
	DevicesMin  int        `json:"devices_min"`
	EndAt       *Timestamp `json:"end_at,omitempty"`
	FacilityIDs []string   `json:"facilities"`
	MaxBidPrice float64    `json:"max_bid_price"`

	Parameters SpotMarketRequestInstanceParameters `json:"instance_parameters"`
}

type SpotMarketRequest struct {
	SpotMarketRequestCreateRequest
	ID         string     `json:"id"`
	Devices    []Device   `json:"devices"`
	Facilities []Facility `json:"facilities"`
	Project    Project    `json:"project"`
	Href       string     `json:"href"`
	Plan       Plan       `json:"plan"`
}

type SpotMarketRequestInstanceParameters struct {
	AlwaysPXE       bool       `json:"always_pxe,omitempty"`
	BillingCycle    string     `json:"billing_cycle"`
	CustomData      string     `json:"customdata,omitempty"`
	Description     string     `json:"description,omitempty"`
	Features        []string   `json:"features,omitempty"`
	Hostname        string     `json:"hostname,omitempty"`
	Hostnames       []string   `json:"hostnames,omitempty"`
	Locked          bool       `json:"locked,omitempty"`
	OperatingSystem string     `json:"operating_system"`
	Plan            string     `json:"plan"`
	ProjectSSHKeys  []string   `json:"project_ssh_keys,omitempty"`
	Tags            []string   `json:"tags"`
	TerminationTime *Timestamp `json:"termination_time,omitempty"`
	UserSSHKeys     []string   `json:"user_ssh_keys,omitempty"`
	UserData        string     `json:"userdata"`
}

type SpotMarketRequestServiceOp struct {
	client *Client
}

func roundPlus(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Floor(f*shift+.5) / shift
}

func (s *SpotMarketRequestServiceOp) Create(cr *SpotMarketRequestCreateRequest, pID string) (*SpotMarketRequest, *Response, error) {
	path := fmt.Sprintf("%s/%s%s?include=devices,project,plan", projectBasePath, pID, spotMarketRequestBasePath)
	cr.MaxBidPrice = roundPlus(cr.MaxBidPrice, 2)
	smr := new(SpotMarketRequest)

	resp, err := s.client.DoRequest("POST", path, cr, smr)
	if err != nil {
		return nil, resp, err
	}

	return smr, resp, err
}

func (s *SpotMarketRequestServiceOp) List(pID string, listOpt *ListOptions) ([]SpotMarketRequest, *Response, error) {
	type smrRoot struct {
		SMRs []SpotMarketRequest `json:"spot_market_requests"`
	}

	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s/%s%s?%s", projectBasePath, pID, spotMarketRequestBasePath, params)
	output := new(smrRoot)

	resp, err := s.client.DoRequest("GET", path, nil, output)
	if err != nil {
		return nil, nil, err
	}

	return output.SMRs, resp, nil
}

func (s *SpotMarketRequestServiceOp) Get(id string, getOpt *GetOptions) (*SpotMarketRequest, *Response, error) {
	params := createGetOptionsURL(getOpt)
	path := fmt.Sprintf("%s/%s?%s", spotMarketRequestBasePath, id, params)
	smr := new(SpotMarketRequest)

	resp, err := s.client.DoRequest("GET", path, nil, &smr)
	if err != nil {
		return nil, resp, err
	}

	return smr, resp, err
}

func (s *SpotMarketRequestServiceOp) Delete(id string, forceDelete bool) (*Response, error) {
	path := fmt.Sprintf("%s/%s", spotMarketRequestBasePath, id)
	var params *map[string]bool
	if forceDelete {
		params = &map[string]bool{"force_termination": true}
	}
	return s.client.DoRequest("DELETE", path, params, nil)
}

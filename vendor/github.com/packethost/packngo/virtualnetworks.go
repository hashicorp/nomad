package packngo

import (
	"fmt"
)

const virtualNetworkBasePath = "/virtual-networks"

// DevicePortService handles operations on a port which belongs to a particular device
type ProjectVirtualNetworkService interface {
	List(projectID string, listOpt *ListOptions) (*VirtualNetworkListResponse, *Response, error)
	Create(*VirtualNetworkCreateRequest) (*VirtualNetwork, *Response, error)
	Get(string, *GetOptions) (*VirtualNetwork, *Response, error)
	Delete(virtualNetworkID string) (*Response, error)
}

type VirtualNetwork struct {
	ID           string  `json:"id"`
	Description  string  `json:"description,omitempty"`
	VXLAN        int     `json:"vxlan,omitempty"`
	FacilityCode string  `json:"facility_code,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
	Href         string  `json:"href"`
	Project      Project `json:"assigned_to"`
}

type ProjectVirtualNetworkServiceOp struct {
	client *Client
}

type VirtualNetworkListResponse struct {
	VirtualNetworks []VirtualNetwork `json:"virtual_networks"`
}

func (i *ProjectVirtualNetworkServiceOp) List(projectID string, listOpt *ListOptions) (*VirtualNetworkListResponse, *Response, error) {

	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s/%s%s?%s", projectBasePath, projectID, virtualNetworkBasePath, params)
	output := new(VirtualNetworkListResponse)

	resp, err := i.client.DoRequest("GET", path, nil, output)
	if err != nil {
		return nil, nil, err
	}

	return output, resp, nil
}

type VirtualNetworkCreateRequest struct {
	ProjectID   string `json:"project_id"`
	Description string `json:"description"`
	Facility    string `json:"facility"`
}

func (i *ProjectVirtualNetworkServiceOp) Get(vlanID string, getOpt *GetOptions) (*VirtualNetwork, *Response, error) {
	params := createGetOptionsURL(getOpt)
	path := fmt.Sprintf("%s/%s?%s", virtualNetworkBasePath, vlanID, params)
	vlan := new(VirtualNetwork)

	resp, err := i.client.DoRequest("GET", path, nil, vlan)
	if err != nil {
		return nil, resp, err
	}

	return vlan, resp, err
}

func (i *ProjectVirtualNetworkServiceOp) Create(input *VirtualNetworkCreateRequest) (*VirtualNetwork, *Response, error) {
	// TODO: May need to add timestamp to output from 'post' request
	// for the 'created_at' attribute of VirtualNetwork struct since
	// API response doesn't include it
	path := fmt.Sprintf("%s/%s%s", projectBasePath, input.ProjectID, virtualNetworkBasePath)
	output := new(VirtualNetwork)

	resp, err := i.client.DoRequest("POST", path, input, output)
	if err != nil {
		return nil, nil, err
	}

	return output, resp, nil
}

func (i *ProjectVirtualNetworkServiceOp) Delete(virtualNetworkID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", virtualNetworkBasePath, virtualNetworkID)

	resp, err := i.client.DoRequest("DELETE", path, nil, nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

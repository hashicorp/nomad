package packngo

import "fmt"

var bgpSessionBasePath = "/bgp/sessions"

// BGPSessionService interface defines available BGP session methods
type BGPSessionService interface {
	Get(string, *GetOptions) (*BGPSession, *Response, error)
	Create(string, CreateBGPSessionRequest) (*BGPSession, *Response, error)
	Delete(string) (*Response, error)
}

type bgpSessionsRoot struct {
	Sessions []BGPSession `json:"bgp_sessions"`
	Meta     meta         `json:"meta"`
}

// BGPSessionServiceOp implements BgpSessionService
type BGPSessionServiceOp struct {
	client *Client
}

// BGPSession represents a Packet BGP Session
type BGPSession struct {
	ID            string   `json:"id,omitempty"`
	Status        string   `json:"status,omitempty"`
	LearnedRoutes []string `json:"learned_routes,omitempty"`
	AddressFamily string   `json:"address_family,omitempty"`
	Device        Device   `json:"device,omitempty"`
	Href          string   `json:"href,omitempty"`
	DefaultRoute  *bool    `json:"default_route,omitempty"`
}

// CreateBGPSessionRequest struct
type CreateBGPSessionRequest struct {
	AddressFamily string `json:"address_family"`
	DefaultRoute  *bool  `json:"default_route,omitempty"`
}

// Create function
func (s *BGPSessionServiceOp) Create(deviceID string, request CreateBGPSessionRequest) (*BGPSession, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", deviceBasePath, deviceID, bgpSessionBasePath)
	session := new(BGPSession)

	resp, err := s.client.DoRequest("POST", path, request, session)
	if err != nil {
		return nil, resp, err
	}

	return session, resp, err
}

// Delete function
func (s *BGPSessionServiceOp) Delete(id string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", bgpSessionBasePath, id)

	return s.client.DoRequest("DELETE", path, nil, nil)
}

// Get function
func (s *BGPSessionServiceOp) Get(id string, getOpt *GetOptions) (session *BGPSession, response *Response, err error) {
	params := createGetOptionsURL(getOpt)
	path := fmt.Sprintf("%s/%s?%s", bgpSessionBasePath, id, params)
	session = new(BGPSession)
	response, err = s.client.DoRequest("GET", path, nil, session)
	if err != nil {
		return nil, response, err
	}

	return session, response, err
}

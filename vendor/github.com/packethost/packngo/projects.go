package packngo

import (
	"fmt"
)

const projectBasePath = "/projects"

// ProjectService interface defines available project methods
type ProjectService interface {
	List(listOpt *ListOptions) ([]Project, *Response, error)
	Get(string, *GetOptions) (*Project, *Response, error)
	Create(*ProjectCreateRequest) (*Project, *Response, error)
	Update(string, *ProjectUpdateRequest) (*Project, *Response, error)
	Delete(string) (*Response, error)
	ListBGPSessions(projectID string, listOpt *ListOptions) ([]BGPSession, *Response, error)
	ListEvents(string, *ListOptions) ([]Event, *Response, error)
}

type projectsRoot struct {
	Projects []Project `json:"projects"`
	Meta     meta      `json:"meta"`
}

// Project represents a Packet project
type Project struct {
	ID              string        `json:"id"`
	Name            string        `json:"name,omitempty"`
	Organization    Organization  `json:"organization,omitempty"`
	Created         string        `json:"created_at,omitempty"`
	Updated         string        `json:"updated_at,omitempty"`
	Users           []User        `json:"members,omitempty"`
	Devices         []Device      `json:"devices,omitempty"`
	SSHKeys         []SSHKey      `json:"ssh_keys,omitempty"`
	URL             string        `json:"href,omitempty"`
	PaymentMethod   PaymentMethod `json:"payment_method,omitempty"`
	BackendTransfer bool          `json:"backend_transfer_enabled"`
}

func (p Project) String() string {
	return Stringify(p)
}

// ProjectCreateRequest type used to create a Packet project
type ProjectCreateRequest struct {
	Name            string `json:"name"`
	PaymentMethodID string `json:"payment_method_id,omitempty"`
	OrganizationID  string `json:"organization_id,omitempty"`
}

func (p ProjectCreateRequest) String() string {
	return Stringify(p)
}

// ProjectUpdateRequest type used to update a Packet project
type ProjectUpdateRequest struct {
	Name            *string `json:"name,omitempty"`
	PaymentMethodID *string `json:"payment_method_id,omitempty"`
	BackendTransfer *bool   `json:"backend_transfer_enabled,omitempty"`
}

func (p ProjectUpdateRequest) String() string {
	return Stringify(p)
}

// ProjectServiceOp implements ProjectService
type ProjectServiceOp struct {
	client *Client
}

// List returns the user's projects
func (s *ProjectServiceOp) List(listOpt *ListOptions) (projects []Project, resp *Response, err error) {
	params := createListOptionsURL(listOpt)
	root := new(projectsRoot)

	path := fmt.Sprintf("%s?%s", projectBasePath, params)

	for {
		resp, err = s.client.DoRequest("GET", path, nil, root)
		if err != nil {
			return nil, resp, err
		}

		projects = append(projects, root.Projects...)

		if root.Meta.Next != nil && (listOpt == nil || listOpt.Page == 0) {
			path = root.Meta.Next.Href
			if params != "" {
				path = fmt.Sprintf("%s&%s", path, params)
			}
			continue
		}

		return
	}
}

// Get returns a project by id
func (s *ProjectServiceOp) Get(projectID string, getOpt *GetOptions) (*Project, *Response, error) {
	params := createGetOptionsURL(getOpt)
	path := fmt.Sprintf("%s/%s?%s", projectBasePath, projectID, params)
	project := new(Project)
	resp, err := s.client.DoRequest("GET", path, nil, project)
	if err != nil {
		return nil, resp, err
	}
	return project, resp, err
}

// Create creates a new project
func (s *ProjectServiceOp) Create(createRequest *ProjectCreateRequest) (*Project, *Response, error) {
	project := new(Project)

	resp, err := s.client.DoRequest("POST", projectBasePath, createRequest, project)
	if err != nil {
		return nil, resp, err
	}

	return project, resp, err
}

// Update updates a project
func (s *ProjectServiceOp) Update(id string, updateRequest *ProjectUpdateRequest) (*Project, *Response, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, id)
	project := new(Project)

	resp, err := s.client.DoRequest("PATCH", path, updateRequest, project)
	if err != nil {
		return nil, resp, err
	}

	return project, resp, err
}

// Delete deletes a project
func (s *ProjectServiceOp) Delete(projectID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, projectID)

	return s.client.DoRequest("DELETE", path, nil, nil)
}

// ListBGPSessions returns all BGP Sessions associated with the project
func (s *ProjectServiceOp) ListBGPSessions(projectID string, listOpt *ListOptions) (bgpSessions []BGPSession, resp *Response, err error) {
	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s/%s%s?%s", projectBasePath, projectID, bgpSessionBasePath, params)

	for {
		subset := new(bgpSessionsRoot)

		resp, err = s.client.DoRequest("GET", path, nil, subset)
		if err != nil {
			return nil, resp, err
		}

		bgpSessions = append(bgpSessions, subset.Sessions...)

		if subset.Meta.Next != nil && (listOpt == nil || listOpt.Page == 0) {
			path = subset.Meta.Next.Href
			if params != "" {
				path = fmt.Sprintf("%s&%s", path, params)
			}
			continue
		}

		return
	}
}

// ListEvents returns list of project events
func (s *ProjectServiceOp) ListEvents(projectID string, listOpt *ListOptions) ([]Event, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", projectBasePath, projectID, eventBasePath)

	return listEvents(s.client, path, listOpt)
}

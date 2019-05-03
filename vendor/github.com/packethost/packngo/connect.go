package packngo

import "fmt"

const (
	connectBasePath = "/packet-connect/connections"
	AzureProviderID = "ed5de8e0-77a9-4d3b-9de0-65281d3aa831"
)

type ConnectService interface {
	List(string, *ListOptions) ([]Connect, *Response, error)
	Get(string, string, *GetOptions) (*Connect, *Response, error)
	Delete(string, string) (*Response, error)
	Create(*ConnectCreateRequest) (*Connect, *Response, error)
	Provision(string, string) (*Connect, *Response, error)
	Deprovision(string, string, bool) (*Connect, *Response, error)
}

type ConnectCreateRequest struct {
	Name            string   `json:"name"`
	ProjectID       string   `json:"project_id"`
	ProviderID      string   `json:"provider_id"`
	ProviderPayload string   `json:"provider_payload"`
	Facility        string   `json:"facility"`
	PortSpeed       int      `json:"port_speed"`
	VLAN            int      `json:"vlan"`
	Tags            []string `json:"tags,omitempty"`
	Description     string   `json:"description,omitempty"`
}

type Connect struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	Name            string `json:"name"`
	ProjectID       string `json:"project_id"`
	ProviderID      string `json:"provider_id"`
	ProviderPayload string `json:"provider_payload"`
	Facility        string `json:"facility"`
	PortSpeed       int    `json:"port_speed"`
	VLAN            int    `json:"vlan"`
	Description     string `json:"description,omitempty"`
}

type ConnectServiceOp struct {
	client *Client
}

type connectsRoot struct {
	Connects []Connect `json:"connections"`
	Meta     meta      `json:"meta"`
}

func (c *ConnectServiceOp) List(projectID string, listOpt *ListOptions) (connects []Connect, resp *Response, err error) {
	params := createListOptionsURL(listOpt)

	project_param := fmt.Sprintf("project_id=%s", projectID)
	if params == "" {
		params = project_param
	} else {
		params = fmt.Sprintf("%s&%s", params, project_param)
	}
	path := fmt.Sprintf("%s/?%s", connectBasePath, params)

	for {
		subset := new(connectsRoot)

		resp, err = c.client.DoRequest("GET", path, nil, subset)
		if err != nil {
			return nil, resp, err
		}

		connects = append(connects, subset.Connects...)

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

func (c *ConnectServiceOp) Deprovision(connectID, projectID string, delete bool) (*Connect, *Response, error) {
	params := fmt.Sprintf("project_id=%s&delete=%t", projectID, delete)
	path := fmt.Sprintf("%s/%s/deprovision?%s", connectBasePath, connectID, params)
	connect := new(Connect)

	resp, err := c.client.DoRequest("POST", path, nil, connect)
	if err != nil {
		return nil, resp, err
	}

	return connect, resp, err
}

func (c *ConnectServiceOp) Provision(connectID, projectID string) (*Connect, *Response, error) {
	params := fmt.Sprintf("project_id=%s", projectID)
	path := fmt.Sprintf("%s/%s/provision?%s", connectBasePath, connectID, params)
	connect := new(Connect)

	resp, err := c.client.DoRequest("POST", path, nil, connect)
	if err != nil {
		return nil, resp, err
	}

	return connect, resp, err
}

func (c *ConnectServiceOp) Create(createRequest *ConnectCreateRequest) (*Connect, *Response, error) {
	url := fmt.Sprintf("%s", connectBasePath)
	connect := new(Connect)

	resp, err := c.client.DoRequest("POST", url, createRequest, connect)
	if err != nil {
		return nil, resp, err
	}

	return connect, resp, err
}

func (c *ConnectServiceOp) Get(connectID, projectID string, getOpt *GetOptions) (*Connect, *Response, error) {
	params := createGetOptionsURL(getOpt)
	project_param := fmt.Sprintf("project_id=%s", projectID)
	if params == "" {
		params = project_param
	} else {
		params = fmt.Sprintf("%s&%s", params, project_param)
	}
	path := fmt.Sprintf("%s/%s?%s", connectBasePath, connectID, params)
	connect := new(Connect)

	resp, err := c.client.DoRequest("GET", path, nil, connect)
	if err != nil {
		return nil, resp, err
	}

	return connect, resp, err
}

func (c *ConnectServiceOp) Delete(connectID, projectID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s?project_id=%s", connectBasePath, connectID,
		projectID)

	return c.client.DoRequest("DELETE", path, nil, nil)
}

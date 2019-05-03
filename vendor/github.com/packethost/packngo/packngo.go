package packngo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	packetTokenEnvVar = "PACKET_AUTH_TOKEN"
	libraryVersion    = "0.1.0"
	baseURL           = "https://api.packet.net/"
	userAgent         = "packngo/" + libraryVersion
	mediaType         = "application/json"
	debugEnvVar       = "PACKNGO_DEBUG"

	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
)

type GetOptions struct {
	Includes []string
	Excludes []string
}

// ListOptions specifies optional global API parameters
type ListOptions struct {
	// for paginated result sets, page of results to retrieve
	Page int `url:"page,omitempty"`
	// for paginated result sets, the number of results to return per page
	PerPage  int `url:"per_page,omitempty"`
	Includes []string
	Excludes []string
}

func makeSureGetOptionsInclude(g *GetOptions, s string) *GetOptions {
	if g == nil {
		return &GetOptions{Includes: []string{s}}
	}
	if !contains(g.Includes, s) {
		g.Includes = append(g.Includes, s)
	}
	return g
}

func makeSureListOptionsInclude(l *ListOptions, s string) *ListOptions {
	if l == nil {
		return &ListOptions{Includes: []string{s}}
	}
	if !contains(l.Includes, s) {
		l.Includes = append(l.Includes, s)
	}
	return l
}

func createGetOptionsURL(g *GetOptions) (url string) {
	if g == nil {
		return ""
	}
	if len(g.Includes) != 0 {
		url += fmt.Sprintf("include=%s", strings.Join(g.Includes, ","))
	}
	if len(g.Excludes) != 0 {
		if url != "" {
			url += "&"
		}
		url += fmt.Sprintf("exclude=%s", strings.Join(g.Excludes, ","))
	}
	return

}

func createListOptionsURL(l *ListOptions) (url string) {
	if l == nil {
		return ""
	}
	if len(l.Includes) != 0 {
		url += fmt.Sprintf("include=%s", strings.Join(l.Includes, ","))
	}
	if len(l.Excludes) != 0 {
		if url != "" {
			url += "&"
		}
		url += fmt.Sprintf("exclude=%s", strings.Join(l.Excludes, ","))
	}
	if l.Page != 0 {
		if url != "" {
			url += "&"
		}
		url += fmt.Sprintf("page=%d", l.Page)
	}

	if l.PerPage != 0 {
		if url != "" {
			url += "&"
		}
		url += fmt.Sprintf("per_page=%d", l.PerPage)
	}

	return
}

// meta contains pagination information
type meta struct {
	Self           *Href `json:"self"`
	First          *Href `json:"first"`
	Last           *Href `json:"last"`
	Previous       *Href `json:"previous,omitempty"`
	Next           *Href `json:"next,omitempty"`
	Total          int   `json:"total"`
	CurrentPageNum int   `json:"current_page"`
	LastPageNum    int   `json:"last_page"`
}

// Response is the http response from api calls
type Response struct {
	*http.Response
	Rate
}

// Href is an API link
type Href struct {
	Href string `json:"href"`
}

func (r *Response) populateRate() {
	// parse the rate limit headers and populate Response.Rate
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		r.Rate.RequestLimit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		r.Rate.RequestsRemaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			r.Rate.Reset = Timestamp{time.Unix(v, 0)}
		}
	}
}

// ErrorResponse is the http response used on errors
type ErrorResponse struct {
	Response    *http.Response
	Errors      []string `json:"errors"`
	SingleError string   `json:"error"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v %v",
		r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode, strings.Join(r.Errors, ", "), r.SingleError)
}

// Client is the base API Client
type Client struct {
	client *http.Client
	debug  bool

	BaseURL *url.URL

	UserAgent     string
	ConsumerToken string
	APIKey        string

	RateLimit Rate

	// Packet Api Objects
	Plans                  PlanService
	Users                  UserService
	Emails                 EmailService
	SSHKeys                SSHKeyService
	Devices                DeviceService
	Projects               ProjectService
	Facilities             FacilityService
	OperatingSystems       OSService
	DeviceIPs              DeviceIPService
	DevicePorts            DevicePortService
	ProjectIPs             ProjectIPService
	ProjectVirtualNetworks ProjectVirtualNetworkService
	Volumes                VolumeService
	VolumeAttachments      VolumeAttachmentService
	SpotMarket             SpotMarketService
	SpotMarketRequests     SpotMarketRequestService
	Organizations          OrganizationService
	BGPSessions            BGPSessionService
	BGPConfig              BGPConfigService
	CapacityService        CapacityService
	Batches                BatchService
	TwoFactorAuth          TwoFactorAuthService
	VPN                    VPNService
	HardwareReservations   HardwareReservationService
	Events                 EventService
	Notifications          NotificationService
	Connects               ConnectService
}

// NewRequest inits a new http request with the proper headers
func (c *Client) NewRequest(method, path string, body interface{}) (*http.Request, error) {
	// relative path to append to the endpoint url, no leading slash please
	rel, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	// json encode the request body, if any
	buf := new(bytes.Buffer)
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Close = true

	req.Header.Add("X-Auth-Token", c.APIKey)
	req.Header.Add("X-Consumer-Token", c.ConsumerToken)

	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", userAgent)
	return req, nil
}

// Do executes the http request
func (c *Client) Do(req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	response := Response{Response: resp}
	response.populateRate()
	if c.debug {
		o, _ := httputil.DumpResponse(response.Response, true)
		log.Printf("\n=======[RESPONSE]============\n%s\n\n", string(o))
	}
	c.RateLimit = response.Rate

	err = checkResponse(resp)
	// if the response is an error, return the ErrorResponse
	if err != nil {
		return &response, err
	}

	if v != nil {
		// if v implements the io.Writer interface, return the raw response
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				return &response, err
			}
		}
	}

	return &response, err
}

// DoRequest is a convenience method, it calls NewRequest followed by Do
// v is the interface to unmarshal the response JSON into
func (c *Client) DoRequest(method, path string, body, v interface{}) (*Response, error) {
	req, err := c.NewRequest(method, path, body)
	if c.debug {
		o, _ := httputil.DumpRequestOut(req, true)
		log.Printf("\n=======[REQUEST]=============\n%s\n", string(o))
	}
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

// DoRequestWithHeader same as DoRequest
func (c *Client) DoRequestWithHeader(method string, headers map[string]string, path string, body, v interface{}) (*Response, error) {
	req, err := c.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	if c.debug {
		o, _ := httputil.DumpRequestOut(req, true)
		log.Printf("\n=======[REQUEST]=============\n%s\n", string(o))
	}
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

// NewClient initializes and returns a Client
func NewClient() (*Client, error) {
	apiToken := os.Getenv(packetTokenEnvVar)
	if apiToken == "" {
		return nil, fmt.Errorf("you must export %s", packetTokenEnvVar)
	}
	c := NewClientWithAuth("packngo lib", apiToken, nil)
	return c, nil

}

// NewClientWithAuth initializes and returns a Client, use this to get an API Client to operate on
// N.B.: Packet's API certificate requires Go 1.5+ to successfully parse. If you are using
// an older version of Go, pass in a custom http.Client with a custom TLS configuration
// that sets "InsecureSkipVerify" to "true"
func NewClientWithAuth(consumerToken string, apiKey string, httpClient *http.Client) *Client {
	client, _ := NewClientWithBaseURL(consumerToken, apiKey, httpClient, baseURL)
	return client
}

// NewClientWithBaseURL returns a Client pointing to nonstandard API URL, e.g.
// for mocking the remote API
func NewClientWithBaseURL(consumerToken string, apiKey string, httpClient *http.Client, apiBaseURL string) (*Client, error) {
	if httpClient == nil {
		// Don't fall back on http.DefaultClient as it's not nice to adjust state
		// implicitly. If the client wants to use http.DefaultClient, they can
		// pass it in explicitly.
		httpClient = &http.Client{}
	}

	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{client: httpClient, BaseURL: u, UserAgent: userAgent, ConsumerToken: consumerToken, APIKey: apiKey}
	c.debug = os.Getenv(debugEnvVar) != ""
	c.Plans = &PlanServiceOp{client: c}
	c.Organizations = &OrganizationServiceOp{client: c}
	c.Users = &UserServiceOp{client: c}
	c.Emails = &EmailServiceOp{client: c}
	c.SSHKeys = &SSHKeyServiceOp{client: c}
	c.Devices = &DeviceServiceOp{client: c}
	c.Projects = &ProjectServiceOp{client: c}
	c.Facilities = &FacilityServiceOp{client: c}
	c.OperatingSystems = &OSServiceOp{client: c}
	c.DeviceIPs = &DeviceIPServiceOp{client: c}
	c.DevicePorts = &DevicePortServiceOp{client: c}
	c.ProjectVirtualNetworks = &ProjectVirtualNetworkServiceOp{client: c}
	c.ProjectIPs = &ProjectIPServiceOp{client: c}
	c.Volumes = &VolumeServiceOp{client: c}
	c.VolumeAttachments = &VolumeAttachmentServiceOp{client: c}
	c.SpotMarket = &SpotMarketServiceOp{client: c}
	c.BGPSessions = &BGPSessionServiceOp{client: c}
	c.BGPConfig = &BGPConfigServiceOp{client: c}
	c.CapacityService = &CapacityServiceOp{client: c}
	c.Batches = &BatchServiceOp{client: c}
	c.TwoFactorAuth = &TwoFactorAuthServiceOp{client: c}
	c.VPN = &VPNServiceOp{client: c}
	c.HardwareReservations = &HardwareReservationServiceOp{client: c}
	c.SpotMarketRequests = &SpotMarketRequestServiceOp{client: c}
	c.Events = &EventServiceOp{client: c}
	c.Notifications = &NotificationServiceOp{client: c}
	c.Connects = &ConnectServiceOp{client: c}

	return c, nil
}

func checkResponse(r *http.Response) error {
	// return if http status code is within 200 range
	if c := r.StatusCode; c >= 200 && c <= 299 {
		// response is good, return
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	// if the response has a body, populate the message in errorResponse
	if err == nil && len(data) > 0 {
		json.Unmarshal(data, errorResponse)
	}

	return errorResponse
}

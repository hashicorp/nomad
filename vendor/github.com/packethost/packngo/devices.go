package packngo

import (
	"encoding/json"
	"fmt"
)

const deviceBasePath = "/devices"

// DeviceService interface defines available device methods
type DeviceService interface {
	List(ProjectID string, listOpt *ListOptions) ([]Device, *Response, error)
	Get(DeviceID string, getOpt *GetOptions) (*Device, *Response, error)
	Create(*DeviceCreateRequest) (*Device, *Response, error)
	Update(string, *DeviceUpdateRequest) (*Device, *Response, error)
	Delete(string) (*Response, error)
	Reboot(string) (*Response, error)
	PowerOff(string) (*Response, error)
	PowerOn(string) (*Response, error)
	Lock(string) (*Response, error)
	Unlock(string) (*Response, error)
	ListBGPSessions(deviceID string, listOpt *ListOptions) ([]BGPSession, *Response, error)
	ListEvents(string, *ListOptions) ([]Event, *Response, error)
}

type devicesRoot struct {
	Devices []Device `json:"devices"`
	Meta    meta     `json:"meta"`
}

// DeviceRaw represents a Packet device from API
type DeviceRaw struct {
	ID                  string                 `json:"id"`
	Href                string                 `json:"href,omitempty"`
	Hostname            string                 `json:"hostname,omitempty"`
	State               string                 `json:"state,omitempty"`
	Created             string                 `json:"created_at,omitempty"`
	Updated             string                 `json:"updated_at,omitempty"`
	Locked              bool                   `json:"locked,omitempty"`
	BillingCycle        string                 `json:"billing_cycle,omitempty"`
	Storage             map[string]interface{} `json:"storage,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Network             []*IPAddressAssignment `json:"ip_addresses"`
	Volumes             []*Volume              `json:"volumes"`
	OS                  *OS                    `json:"operating_system,omitempty"`
	Plan                *Plan                  `json:"plan,omitempty"`
	Facility            *Facility              `json:"facility,omitempty"`
	Project             *Project               `json:"project,omitempty"`
	ProvisionEvents     []*Event               `json:"provisioning_events,omitempty"`
	ProvisionPer        float32                `json:"provisioning_percentage,omitempty"`
	UserData            string                 `json:"userdata,omitempty"`
	RootPassword        string                 `json:"root_password,omitempty"`
	IPXEScriptURL       string                 `json:"ipxe_script_url,omitempty"`
	AlwaysPXE           bool                   `json:"always_pxe,omitempty"`
	HardwareReservation Href                   `json:"hardware_reservation,omitempty"`
	SpotInstance        bool                   `json:"spot_instance,omitempty"`
	SpotPriceMax        float64                `json:"spot_price_max,omitempty"`
	TerminationTime     *Timestamp             `json:"termination_time,omitempty"`
	NetworkPorts        []Port                 `json:"network_ports,omitempty"`
	CustomData          map[string]interface{} `json:"customdata,omitempty"`
	SSHKeys             []SSHKey               `json:"ssh_keys,omitempty"`
}

type Device struct {
	DeviceRaw
	NetworkType string
}

func (d *Device) UnmarshalJSON(b []byte) error {
	dJSON := DeviceRaw{}
	if err := json.Unmarshal(b, &dJSON); err != nil {
		return err
	}
	d.DeviceRaw = dJSON
	if len(dJSON.NetworkPorts) > 0 {
		networkType, err := dJSON.GetNetworkType()
		if err != nil {
			return err
		}
		d.NetworkType = networkType
	}
	return nil
}

type NetworkInfo struct {
	PublicIPv4  string
	PublicIPv6  string
	PrivateIPv4 string
}

func (d *Device) GetNetworkInfo() NetworkInfo {
	ni := NetworkInfo{}
	for _, ip := range d.Network {
		// Initial device IPs are fixed and marked as "Management"
		if ip.Management {
			if ip.AddressFamily == 4 {
				if ip.Public {
					ni.PublicIPv4 = ip.Address
				} else {
					ni.PrivateIPv4 = ip.Address
				}
			} else {
				ni.PublicIPv6 = ip.Address
			}
		}
	}
	return ni
}

func (d Device) String() string {
	return Stringify(d)
}

func (d DeviceRaw) GetNetworkType() (string, error) {
	if len(d.NetworkPorts) == 0 {
		return "", fmt.Errorf("Device has no network ports listed")
	}
	for _, p := range d.NetworkPorts {
		if p.Name == "bond0" {
			return p.NetworkType, nil
		}
	}
	return "", fmt.Errorf("Bound port not found")
}

// DeviceCreateRequest type used to create a Packet device
type DeviceCreateRequest struct {
	Hostname              string     `json:"hostname"`
	Plan                  string     `json:"plan"`
	Facility              []string   `json:"facility"`
	OS                    string     `json:"operating_system"`
	BillingCycle          string     `json:"billing_cycle"`
	ProjectID             string     `json:"project_id"`
	UserData              string     `json:"userdata"`
	Storage               string     `json:"storage,omitempty"`
	Tags                  []string   `json:"tags"`
	IPXEScriptURL         string     `json:"ipxe_script_url,omitempty"`
	PublicIPv4SubnetSize  int        `json:"public_ipv4_subnet_size,omitempty"`
	AlwaysPXE             bool       `json:"always_pxe,omitempty"`
	HardwareReservationID string     `json:"hardware_reservation_id,omitempty"`
	SpotInstance          bool       `json:"spot_instance,omitempty"`
	SpotPriceMax          float64    `json:"spot_price_max,omitempty,string"`
	TerminationTime       *Timestamp `json:"termination_time,omitempty"`
	CustomData            string     `json:"customdata,omitempty"`
	// UserSSHKeys is a list of user UUIDs - essentialy a list of
	// collaborators. The users must be a collaborator in the same project
	// where the device is created. The user's SSH keys then go to the
	// device.
	UserSSHKeys []string `json:"user_ssh_keys,omitempty"`
	// Project SSHKeys is a list of SSHKeys resource UUIDs. If this param
	// is supplied, only the listed SSHKeys will go to the device.
	// Any other Project SSHKeys and any User SSHKeys will not be present
	// in the device.
	ProjectSSHKeys []string          `json:"project_ssh_keys,omitempty"`
	Features       map[string]string `json:"features,omitempty"`
}

// DeviceUpdateRequest type used to update a Packet device
type DeviceUpdateRequest struct {
	Hostname      *string   `json:"hostname,omitempty"`
	Description   *string   `json:"description,omitempty"`
	UserData      *string   `json:"userdata,omitempty"`
	Locked        *bool     `json:"locked,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
	AlwaysPXE     *bool     `json:"always_pxe,omitempty"`
	IPXEScriptURL *string   `json:"ipxe_script_url,omitempty"`
	CustomData    *string   `json:"customdata,omitempty"`
}

func (d DeviceCreateRequest) String() string {
	return Stringify(d)
}

// DeviceActionRequest type used to execute actions on devices
type DeviceActionRequest struct {
	Type string `json:"type"`
}

func (d DeviceActionRequest) String() string {
	return Stringify(d)
}

// DeviceServiceOp implements DeviceService
type DeviceServiceOp struct {
	client *Client
}

// List returns devices on a project
func (s *DeviceServiceOp) List(projectID string, listOpt *ListOptions) (devices []Device, resp *Response, err error) {
	listOpt = makeSureListOptionsInclude(listOpt, "facility")
	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s/%s%s?%s", projectBasePath, projectID, deviceBasePath, params)

	for {
		subset := new(devicesRoot)

		resp, err = s.client.DoRequest("GET", path, nil, subset)
		if err != nil {
			return nil, resp, err
		}

		devices = append(devices, subset.Devices...)

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

// Get returns a device by id
func (s *DeviceServiceOp) Get(deviceID string, getOpt *GetOptions) (*Device, *Response, error) {
	getOpt = makeSureGetOptionsInclude(getOpt, "facility")
	params := createGetOptionsURL(getOpt)

	path := fmt.Sprintf("%s/%s?%s", deviceBasePath, deviceID, params)
	device := new(Device)
	resp, err := s.client.DoRequest("GET", path, nil, device)
	if err != nil {
		return nil, resp, err
	}
	return device, resp, err
}

// Create creates a new device
func (s *DeviceServiceOp) Create(createRequest *DeviceCreateRequest) (*Device, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", projectBasePath, createRequest.ProjectID, deviceBasePath)
	device := new(Device)

	resp, err := s.client.DoRequest("POST", path, createRequest, device)
	if err != nil {
		return nil, resp, err
	}
	return device, resp, err
}

// Update updates an existing device
func (s *DeviceServiceOp) Update(deviceID string, updateRequest *DeviceUpdateRequest) (*Device, *Response, error) {
	path := fmt.Sprintf("%s/%s?include=facility", deviceBasePath, deviceID)
	device := new(Device)

	resp, err := s.client.DoRequest("PUT", path, updateRequest, device)
	if err != nil {
		return nil, resp, err
	}

	return device, resp, err
}

// Delete deletes a device
func (s *DeviceServiceOp) Delete(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", deviceBasePath, deviceID)

	return s.client.DoRequest("DELETE", path, nil, nil)
}

// Reboot reboots on a device
func (s *DeviceServiceOp) Reboot(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s/actions", deviceBasePath, deviceID)
	action := &DeviceActionRequest{Type: "reboot"}

	return s.client.DoRequest("POST", path, action, nil)
}

// PowerOff powers on a device
func (s *DeviceServiceOp) PowerOff(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s/actions", deviceBasePath, deviceID)
	action := &DeviceActionRequest{Type: "power_off"}

	return s.client.DoRequest("POST", path, action, nil)
}

// PowerOn powers on a device
func (s *DeviceServiceOp) PowerOn(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s/actions", deviceBasePath, deviceID)
	action := &DeviceActionRequest{Type: "power_on"}

	return s.client.DoRequest("POST", path, action, nil)
}

type lockType struct {
	Locked bool `json:"locked"`
}

// Lock sets a device to "locked"
func (s *DeviceServiceOp) Lock(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", deviceBasePath, deviceID)
	action := lockType{Locked: true}

	return s.client.DoRequest("PATCH", path, action, nil)
}

// Unlock sets a device to "unlocked"
func (s *DeviceServiceOp) Unlock(deviceID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", deviceBasePath, deviceID)
	action := lockType{Locked: false}

	return s.client.DoRequest("PATCH", path, action, nil)
}

// ListBGPSessions returns all BGP Sessions associated with the device
func (s *DeviceServiceOp) ListBGPSessions(deviceID string, listOpt *ListOptions) (bgpSessions []BGPSession, resp *Response, err error) {
	params := createListOptionsURL(listOpt)
	path := fmt.Sprintf("%s/%s%s?%s", deviceBasePath, deviceID, bgpSessionBasePath, params)

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

// ListEvents returns list of device events
func (s *DeviceServiceOp) ListEvents(deviceID string, listOpt *ListOptions) ([]Event, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", deviceBasePath, deviceID, eventBasePath)

	return listEvents(s.client, path, listOpt)
}

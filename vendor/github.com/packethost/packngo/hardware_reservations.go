package packngo

import "fmt"

const hardwareReservationBasePath = "/hardware-reservations"

// HardwareReservationService interface defines available hardware reservation functions
type HardwareReservationService interface {
	Get(hardwareReservationID string, getOpt *GetOptions) (*HardwareReservation, *Response, error)
	List(projectID string, listOpt *ListOptions) ([]HardwareReservation, *Response, error)
	Move(string, string) (*HardwareReservation, *Response, error)
}

// HardwareReservationServiceOp implements HardwareReservationService
type HardwareReservationServiceOp struct {
	client *Client
}

// HardwareReservation struct
type HardwareReservation struct {
	ID            string    `json:"id,omitempty"`
	ShortID       string    `json:"short_id,omitempty"`
	Facility      Facility  `json:"facility,omitempty"`
	Plan          Plan      `json:"plan,omitempty"`
	Provisionable bool      `json:"provisionable,omitempty"`
	Spare         bool      `json:"spare,omitempty"`
	SwitchUUID    string    `json:"switch_uuid,omitempty"`
	Intervals     int       `json:"intervals,omitempty"`
	CurrentPeriod int       `json:"current_period,omitempty"`
	Href          string    `json:"href,omitempty"`
	Project       Project   `json:"project,omitempty"`
	Device        *Device   `json:"device,omitempty"`
	CreatedAt     Timestamp `json:"created_at,omitempty"`
}

type hardwareReservationRoot struct {
	HardwareReservations []HardwareReservation `json:"hardware_reservations"`
	Meta                 meta                  `json:"meta"`
}

// List returns all hardware reservations for a given project
func (s *HardwareReservationServiceOp) List(projectID string, listOpt *ListOptions) (reservations []HardwareReservation, resp *Response, err error) {
	root := new(hardwareReservationRoot)
	params := createListOptionsURL(listOpt)

	path := fmt.Sprintf("%s/%s%s?%s", projectBasePath, projectID, hardwareReservationBasePath, params)

	for {
		subset := new(hardwareReservationRoot)

		resp, err = s.client.DoRequest("GET", path, nil, root)
		if err != nil {
			return nil, resp, err
		}

		reservations = append(reservations, root.HardwareReservations...)

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

// Get returns a single hardware reservation
func (s *HardwareReservationServiceOp) Get(hardwareReservationdID string, getOpt *GetOptions) (*HardwareReservation, *Response, error) {
	params := createGetOptionsURL(getOpt)

	hardwareReservation := new(HardwareReservation)

	path := fmt.Sprintf("%s/%s?%s", hardwareReservationBasePath, hardwareReservationdID, params)

	resp, err := s.client.DoRequest("GET", path, nil, hardwareReservation)
	if err != nil {
		return nil, resp, err
	}

	return hardwareReservation, resp, err
}

// Move a hardware reservation to another project
func (s *HardwareReservationServiceOp) Move(hardwareReservationdID, projectID string) (*HardwareReservation, *Response, error) {
	hardwareReservation := new(HardwareReservation)
	path := fmt.Sprintf("%s/%s/%s", hardwareReservationBasePath, hardwareReservationdID, "move")
	body := map[string]string{}
	body["project_id"] = projectID

	resp, err := s.client.DoRequest("POST", path, body, hardwareReservation)
	if err != nil {
		return nil, resp, err
	}

	return hardwareReservation, resp, err
}

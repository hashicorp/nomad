package packngo

import "fmt"

const eventBasePath = "/events"

// Event struct
type Event struct {
	ID            string     `json:"id,omitempty"`
	State         string     `json:"state,omitempty"`
	Type          string     `json:"type,omitempty"`
	Body          string     `json:"body,omitempty"`
	Relationships []Href     `json:"relationships,omitempty"`
	Interpolated  string     `json:"interpolated,omitempty"`
	CreatedAt     *Timestamp `json:"created_at,omitempty"`
	Href          string     `json:"href,omitempty"`
}

type eventsRoot struct {
	Events []Event `json:"events,omitempty"`
	Meta   meta    `json:"meta,omitempty"`
}

// EventService interface defines available event functions
type EventService interface {
	List(*ListOptions) ([]Event, *Response, error)
	Get(string, *GetOptions) (*Event, *Response, error)
}

// EventServiceOp implements EventService
type EventServiceOp struct {
	client *Client
}

// List returns all events
func (s *EventServiceOp) List(listOpt *ListOptions) ([]Event, *Response, error) {
	return listEvents(s.client, eventBasePath, listOpt)
}

// Get returns an event by ID
func (s *EventServiceOp) Get(eventID string, getOpt *GetOptions) (*Event, *Response, error) {
	path := fmt.Sprintf("%s/%s", eventBasePath, eventID)
	return get(s.client, path, getOpt)
}

// list helper function for all event functions
func listEvents(client *Client, path string, listOpt *ListOptions) (events []Event, resp *Response, err error) {
	params := createListOptionsURL(listOpt)
	path = fmt.Sprintf("%s?%s", path, params)

	for {
		subset := new(eventsRoot)

		resp, err = client.DoRequest("GET", path, nil, subset)
		if err != nil {
			return nil, resp, err
		}

		events = append(events, subset.Events...)

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

// list helper function for all event functions
/*
func listEvents(client *Client, path string, listOpt *ListOptions) ([]Event, *Response, error) {
	params := createListOptionsURL(listOpt)
	root := new(eventsRoot)

	path = fmt.Sprintf("%s?%s", path, params)

	resp, err := client.DoRequest("GET", path, nil, root)
	if err != nil {
		return nil, resp, err
	}

	return root.Events, resp, err
}
*/

func get(client *Client, path string, getOpt *GetOptions) (*Event, *Response, error) {
	params := createGetOptionsURL(getOpt)

	event := new(Event)

	path = fmt.Sprintf("%s?%s", path, params)

	resp, err := client.DoRequest("GET", path, nil, event)
	if err != nil {
		return nil, resp, err
	}

	return event, resp, err
}

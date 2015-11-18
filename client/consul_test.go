package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"os"
	"testing"
	"time"
)

func TestMakeChecks(t *testing.T) {
	service := &structs.Service{
		Id:   "Foo",
		Name: "Bar",
		Checks: []structs.ServiceCheck{
			{
				Type:     "http",
				Path:     "/foo/bar",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
			{
				Type:     "http",
				Protocol: "https",
				Path:     "/foo/bar",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
			{
				Type:     "tcp",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
		},
	}

	logger := log.New(os.Stdout, "logger: ", log.Lshortfile)

	c, _ := NewConsulClient(logger, "")
	checks := c.makeChecks(service, "10.10.0.1", 8090)

	if checks[0].HTTP != "http://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", checks[0].HTTP)
	}

	if checks[1].HTTP != "https://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", checks[0].HTTP)
	}

	if checks[2].TCP != "10.10.0.1:8090" {
		t.Fatalf("Invalid tcp check: %v", checks[0].TCP)
	}
}

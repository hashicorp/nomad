// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration/checks/checkstore"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

var (
	_ interfaces.RunnerPrerunHook  = (*checksHook)(nil)
	_ interfaces.RunnerUpdateHook  = (*checksHook)(nil)
	_ interfaces.RunnerPreKillHook = (*checksHook)(nil)
)

func makeCheckStore(logger hclog.Logger) checkstore.Shim {
	db := state.NewMemDB(logger)
	checkStore := checkstore.NewStore(logger, db)
	return checkStore
}

func allocWithNomadChecks(addr, port string, onGroup bool) *structs.Allocation {
	alloc := mock.Alloc()
	group := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	task := "task-one"
	if onGroup {
		task = ""
	}

	services := []*structs.Service{
		{
			Name:        "service-one",
			TaskName:    "web",
			PortLabel:   port,
			AddressMode: "auto",
			Address:     addr,
			Provider:    "nomad",
			Checks: []*structs.ServiceCheck{
				{
					Name:        "check-ok",
					Type:        "http",
					Path:        "/",
					Protocol:    "http",
					PortLabel:   port,
					AddressMode: "auto",
					Interval:    250 * time.Millisecond,
					Timeout:     1 * time.Second,
					Method:      "GET",
					TaskName:    task,
				},
				{
					Name:        "check-error",
					Type:        "http",
					Path:        "/fail",
					Protocol:    "http",
					PortLabel:   port,
					AddressMode: "auto",
					Interval:    250 * time.Millisecond,
					Timeout:     1 * time.Second,
					Method:      "GET",
					TaskName:    task,
				},
				{
					Name:        "check-hang",
					Type:        "http",
					Path:        "/hang",
					Protocol:    "http",
					PortLabel:   port,
					AddressMode: "auto",
					Interval:    250 * time.Millisecond,
					Timeout:     500 * time.Millisecond,
					Method:      "GET",
					TaskName:    task,
				},
			},
		},
	}

	switch onGroup {
	case true:
		group.Tasks[0].Services = nil
		group.Services = services
	case false:
		group.Services = nil
		group.Tasks[0].Services = services
	}
	return alloc
}

func allocWithDifferentNomadChecks(id, addr, port string) *structs.Allocation {
	alloc := allocWithNomadChecks(addr, port, true)
	alloc.ID = id
	group := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	group.Services[0].Checks[2].Path = "/" // the hanging check is now ok

	// append 4th check, this one is failing
	group.Services[0].Checks = append(group.Services[0].Checks, &structs.ServiceCheck{
		Name:        "check-error-2",
		Type:        "http",
		Path:        "/fail",
		Protocol:    "http",
		PortLabel:   port,
		AddressMode: "auto",
		Interval:    250 * time.Millisecond,
		Timeout:     1 * time.Second,
		Method:      "GET",
	})
	return alloc
}

var checkHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/fail":
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "500 problem")
	case "/hang":
		time.Sleep(2 * time.Second)
		_, _ = io.WriteString(w, "too slow")
	default:
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "200 ok")
	}
})

func TestCheckHook_Checks_ResultsSet(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// create an http server with various responses
	ts := httptest.NewServer(checkHandler)
	defer ts.Close()

	cases := []struct {
		name    string
		onGroup bool
	}{
		{name: "group-level", onGroup: true},
		{name: "task-level", onGroup: false},
	}

	for _, tc := range cases {
		checkStore := makeCheckStore(logger)

		// get the address and port for http server
		tokens := strings.Split(ts.URL, ":")
		addr, port := strings.TrimPrefix(tokens[1], "//"), tokens[2]

		network := mock.NewNetworkStatus(addr)

		alloc := allocWithNomadChecks(addr, port, tc.onGroup)

		h := newChecksHook(logger, alloc, checkStore, network)

		// initialize is called; observers are created but not started yet
		must.MapEmpty(t, h.observers)

		// calling pre-run starts the observers
		err := h.Prerun()
		must.NoError(t, err)

		testutil.WaitForResultUntil(
			2*time.Second,
			func() (bool, error) {
				results := checkStore.List(alloc.ID)
				passing, failing, pending := 0, 0, 0
				for _, result := range results {
					switch result.Status {
					case structs.CheckSuccess:
						passing++
					case structs.CheckFailure:
						failing++
					case structs.CheckPending:
						pending++
					}
				}
				if passing != 1 || failing != 2 || pending != 0 {
					fmt.Printf("results %v\n", results)
					return false, fmt.Errorf(
						"expected 1 passing, 2 failing, 0 pending, got %d passing, %d failing, %d pending",
						passing, failing, pending,
					)
				}
				return true, nil
			},
			func(err error) {
				t.Fatalf(err.Error())
			},
		)

		h.PreKill() // stop observers, cleanup

		// assert shim no longer contains results for the alloc
		results := checkStore.List(alloc.ID)
		must.MapEmpty(t, results)
	}
}

func TestCheckHook_Checks_UpdateSet(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// create an http server with various responses
	ts := httptest.NewServer(checkHandler)
	defer ts.Close()

	// get the address and port for http server
	tokens := strings.Split(ts.URL, ":")
	addr, port := strings.TrimPrefix(tokens[1], "//"), tokens[2]

	shim := makeCheckStore(logger)

	network := mock.NewNetworkStatus(addr)

	alloc := allocWithNomadChecks(addr, port, true)

	h := newChecksHook(logger, alloc, shim, network)

	// calling pre-run starts the observers
	err := h.Prerun()
	must.NoError(t, err)

	// initial set of checks
	testutil.WaitForResultUntil(
		2*time.Second,
		func() (bool, error) {
			results := shim.List(alloc.ID)
			passing, failing, pending := 0, 0, 0
			for _, result := range results {
				switch result.Status {
				case structs.CheckSuccess:
					passing++
				case structs.CheckFailure:
					failing++
				case structs.CheckPending:
					pending++
				}
			}
			if passing != 1 || failing != 2 || pending != 0 {
				fmt.Printf("results %v\n", results)
				return false, fmt.Errorf(
					"(initial set) expected 1 passing, 2 failing, 0 pending, got %d passing, %d failing, %d pending",
					passing, failing, pending,
				)
			}
			return true, nil
		},
		func(err error) {
			t.Fatalf(err.Error())
		},
	)

	request := &interfaces.RunnerUpdateRequest{
		Alloc: allocWithDifferentNomadChecks(alloc.ID, addr, port),
	}

	err = h.Update(request)
	must.NoError(t, err)

	// updated set of checks
	testutil.WaitForResultUntil(
		2*time.Second,
		func() (bool, error) {
			results := shim.List(alloc.ID)
			passing, failing, pending := 0, 0, 0
			for _, result := range results {
				switch result.Status {
				case structs.CheckSuccess:
					passing++
				case structs.CheckFailure:
					failing++
				case structs.CheckPending:
					pending++
				}
			}
			if passing != 2 || failing != 2 || pending != 0 {
				fmt.Printf("results %v\n", results)
				return false, fmt.Errorf(
					"(updated set) expected 2 passing, 2 failing, 0 pending, got %d passing, %d failing, %d pending",
					passing, failing, pending,
				)
			}
			return true, nil
		},
		func(err error) {
			t.Fatalf(err.Error())
		},
	)

	h.PreKill() // stop observers, cleanup

	// assert shim no longer contains results for the alloc
	results := shim.List(alloc.ID)
	must.MapEmpty(t, results)
}

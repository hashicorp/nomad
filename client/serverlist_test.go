package client

import (
	"log"
	"os"
	"strings"
	"testing"
)

func TestServerList(t *testing.T) {
	t.Parallel()
	s := newServerList()

	// New lists should be empty
	if e := s.all(); len(e) != 0 {
		t.Fatalf("expected empty list to return an empty list, but received: %+q", e)
	}

	mklist := func() endpoints {
		return endpoints{
			&endpoint{"b", nil, 1},
			&endpoint{"c", nil, 1},
			&endpoint{"g", nil, 2},
			&endpoint{"d", nil, 1},
			&endpoint{"e", nil, 1},
			&endpoint{"f", nil, 1},
			&endpoint{"h", nil, 2},
			&endpoint{"a", nil, 0},
		}
	}
	s.set(mklist())

	orig := mklist()
	all := s.all()
	if len(all) != len(orig) {
		t.Fatalf("expected %d endpoints but only have %d", len(orig), len(all))
	}

	// Assert list is properly randomized+sorted
	for i, pri := range []int{0, 1, 1, 1, 1, 1, 2, 2} {
		if all[i].priority != pri {
			t.Errorf("expected endpoint %d (%+q) to be priority %d", i, all[i], pri)
		}
	}

	// Subsequent sets should reshuffle (try multiple times as they may
	// shuffle in the same order)
	tries := 0
	max := 3
	for ; tries < max; tries++ {
		if s.all().String() == s.all().String() {
			// eek, matched; try again in case we just got unlucky
			continue
		}
		break
	}
	if tries == max {
		t.Fatalf("after %d attempts servers were still not random reshuffled", tries)
	}

	// Mark an endpoint as failed enough that it should be at the end of the list
	sa := &endpoint{"a", nil, 0}
	s.failed(sa)
	s.failed(sa)
	s.failed(sa)
	all2 := s.all()
	if len(all2) != len(orig) {
		t.Fatalf("marking should not have changed list length")
	}
	if all2[len(all)-1].name != sa.name {
		t.Fatalf("failed endpoint should be at end of list: %+q", all2)
	}

	// But if the bad endpoint succeeds even once it should be bumped to the top group
	s.good(sa)
	found := false
	for _, e := range s.all() {
		if e.name == sa.name {
			if e.priority != 0 {
				t.Fatalf("server newly marked good should have highest priority")
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("what happened to endpoint A?!")
	}
}

// TestClient_ServerList tests client methods that interact with the internal
// nomad server list.
func TestClient_ServerList(t *testing.T) {
	t.Parallel()
	// manually create a mostly empty client to avoid spinning up a ton of
	// goroutines that complicate testing
	client := Client{servers: newServerList(), logger: log.New(os.Stderr, "", log.Ltime|log.Lshortfile)}

	if s := client.GetServers(); len(s) != 0 {
		t.Fatalf("expected server lit to be empty but found: %+q", s)
	}
	if err := client.SetServers(nil); err != noServersErr {
		t.Fatalf("expected setting an empty list to return a 'no servers' error but received %v", err)
	}
	if err := client.SetServers([]string{"123.456.13123.123.13:80"}); err == nil {
		t.Fatalf("expected setting a bad server to return an error")
	}
	if err := client.SetServers([]string{"123.456.13123.123.13:80", "127.0.0.1:1234", "127.0.0.1"}); err != nil {
		t.Fatalf("expected setting at least one good server to succeed but received: %v", err)
	}
	s := client.GetServers()
	if len(s) != 2 {
		t.Fatalf("expected 2 servers but received: %+q", s)
	}
	for _, host := range s {
		if !strings.HasPrefix(host, "127.0.0.1:") {
			t.Errorf("expected both servers to be localhost and include port but found: %s", host)
		}
	}
}

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/version"
)

func TestHTTP_StatusLeader(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/status/leader", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.StatusLeaderRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		leader := obj.(string)
		if leader == "" {
			t.Fatalf("bad: %#v", leader)
		}
	})
}

func TestHTTP_StatusPeers(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/status/peers", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.StatusPeersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the job
		peers := obj.([]string)
		if len(peers) == 0 {
			t.Fatalf("bad: %#v", peers)
		}
	})
}

func TestHTTP_StatusVersion(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/status/version", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.StatusVersionRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the version
		serverVersion := obj.(string)
		clientVersion := version.GetVersion().FullVersionNumber(true)
		if serverVersion != clientVersion {
			t.Fatalf("bad: version mismatch %s vs %s", serverVersion, clientVersion)
		}
	})
}

package testutil

import (
	"net/http/httptest"
	"testing"
)

func ValidateMetaResponseHeaders(respW *httptest.ResponseRecorder, t *testing.T) {
	// Check for the index
	if respW.HeaderMap.Get("X-Nomad-Index") == "" {
		t.Fatalf("missing index")
	}
	if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
		t.Fatalf("missing known leader")
	}
	if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
		t.Fatalf("missing last contact")
	}
}

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP_FSDirectoryList(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/ls", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.DirectoryListRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHTTP_FSStat(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stat/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

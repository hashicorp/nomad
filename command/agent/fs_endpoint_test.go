package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
		if !strings.HasPrefix(err.Error(), "alloc id not found") {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if !strings.HasPrefix(err.Error(), "must provide a file name") {
			t.Fatal("expected error")
		}
	})
}

func TestHTTP_FSReadAt(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/readat/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo?path=/path/to/file", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

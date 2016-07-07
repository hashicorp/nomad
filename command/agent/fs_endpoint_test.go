package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllocDirFS_List_MissingParams(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/ls/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.DirectoryListRequest(respW, req)
		if err != allocIDNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}
	})
}

func TestAllocDirFS_Stat_MissingParams(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stat/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err != allocIDNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err != fileNameNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}

	})
}

func TestAllocDirFS_ReadAt_MissingParams(t *testing.T) {
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

func TestHTTP_FsStream_EOF_Modify(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
	})
}

package logdaemon

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApi_RegisterAndDeRegisterTask(t *testing.T) {
	ld, err := NewLogDaemon(NewLogDaemonConfig())
	if err != nil {
		t.Errorf("Error in creating log daemon: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(ld.Tasks))
	defer ts.Close()

	trackedTaskReq := trackedTask{Name: "redis", AllocId: "123", Driver: "docker"}
	js, _ := json.Marshal(trackedTaskReq)

	resp, err := http.Post(ts.URL, "application/json", bytes.NewBuffer(js))
	if err != nil {
		t.Errorf("Error while registering new task: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status code: %v, Actual: %v, Body: %v", http.StatusCreated,
			resp.StatusCode, string(body))
	}

	if len(ld.tasks) != 1 {
		t.Fatalf("Expected number of registered tasks: %d, Actual: %d", 1, len(ld.tasks))
	}

	req, _ := http.NewRequest("DELETE", ts.URL, bytes.NewBuffer(js))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Error in making delete request for task: %v", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status code: %v, Actual: %v, Body: %v", http.StatusAccepted,
			resp.StatusCode, body)
	}

	if len(ld.tasks) != 0 {
		t.Fatalf("Expected number of registered tasks: %d, Actual: %d", 1, len(ld.tasks))
	}

}

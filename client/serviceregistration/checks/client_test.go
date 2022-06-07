package checks

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/freeport"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"oss.indeed.com/go/libtime/libtimetest"
)

func TestChecker_Do_HTTP(t *testing.T) {
	ci.Parallel(t)

	// an example response that will be truncated
	tooLong, truncate := bigResponse()

	// create an http server with various responses
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fail":
			w.WriteHeader(500)
			_, _ = io.WriteString(w, "500 problem")
		case "/hang":
			time.Sleep(1 * time.Second)
			_, _ = io.WriteString(w, "too slow")
		case "/long-fail":
			w.WriteHeader(500)
			_, _ = io.WriteString(w, tooLong)
		case "/long-not-fail":
			w.WriteHeader(201)
			_, _ = io.WriteString(w, tooLong)
		default:
			w.WriteHeader(200)
			_, _ = io.WriteString(w, "200 ok")
		}
	}))
	defer ts.Close()

	// get the address and port for http server
	tokens := strings.Split(ts.URL, ":")
	addr, port := strings.TrimPrefix(tokens[1], "//"), tokens[2]

	// create a mock clock so we can assert time is set
	now := time.Date(2022, 1, 2, 3, 4, 5, 6, time.UTC)
	clock := libtimetest.NewClockMock(t).NowMock.Return(now)

	makeQueryContext := func() *QueryContext {
		return &QueryContext{
			ID:               "abc123",
			CustomAddress:    addr,
			ServicePortLabel: port,
			Networks:         nil,
			NetworkStatus:    mock.NewNetworkStatus(addr),
			Ports:            nil,
			Group:            "group",
			Task:             "task",
			Service:          "service",
			Check:            "check",
		}
	}

	makeQuery := func(
		kind structs.CheckMode,
		path string,
	) *Query {
		return &Query{
			Mode:        kind,
			Type:        "http",
			Timeout:     100 * time.Millisecond,
			AddressMode: "auto",
			PortLabel:   port,
			Protocol:    "http",
			Path:        path,
			Method:      "GET",
		}
	}

	makeExpResult := func(
		kind structs.CheckMode,
		status structs.CheckStatus,
		code int,
		output string,
	) *structs.CheckQueryResult {
		return &structs.CheckQueryResult{
			ID:         "abc123",
			Mode:       kind,
			Status:     status,
			StatusCode: code,
			Output:     output,
			Timestamp:  now.Unix(),
			Group:      "group",
			Task:       "task",
			Service:    "service",
			Check:      "check",
		}
	}

	cases := []struct {
		name      string
		qc        *QueryContext
		q         *Query
		expResult *structs.CheckQueryResult
	}{{
		name: "200 healthiness",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Healthiness, "/"),
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckSuccess,
			http.StatusOK,
			"nomad: http ok",
		),
	}, {
		name: "200 readiness",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Readiness, "/"),
		expResult: makeExpResult(
			structs.Readiness,
			structs.CheckSuccess,
			http.StatusOK,
			"nomad: http ok",
		),
	}, {
		name: "500 healthiness",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Healthiness, "fail"),
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckFailure,
			http.StatusInternalServerError,
			"500 problem",
		),
	}, {
		name: "hang",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Healthiness, "hang"),
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckFailure,
			0,
			fmt.Sprintf(`nomad: Get "%s/hang": context deadline exceeded`, ts.URL),
		),
	}, {
		name: "500 truncate",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Healthiness, "long-fail"),
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckFailure,
			http.StatusInternalServerError,
			truncate,
		),
	}, {
		name: "201 truncate",
		qc:   makeQueryContext(),
		q:    makeQuery(structs.Healthiness, "long-not-fail"),
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckSuccess,
			http.StatusCreated,
			truncate,
		),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testlog.HCLogger(t)

			c := New(logger)
			c.(*checker).clock = clock

			ctx := context.Background()
			result := c.Do(ctx, tc.qc, tc.q)
			must.Eq(t, tc.expResult, result)
		})
	}
}

// bigResponse creates a response payload larger than the maximum outputSizeLimit
// as well as the same response but truncated to length of outputSizeLimit
func bigResponse() (string, string) {
	size := outputSizeLimit + 5
	b := make([]byte, size, size)
	for i := 0; i < size; i++ {
		b[i] = 'a'
	}
	s := string(b)
	return s, s[:outputSizeLimit]
}

func TestChecker_Do_TCP(t *testing.T) {
	ci.Parallel(t)

	// create a mock clock so we can assert time is set
	now := time.Date(2022, 1, 2, 3, 4, 5, 6, time.UTC)
	clock := libtimetest.NewClockMock(t).NowMock.Return(now)

	makeQueryContext := func(address string, port int) *QueryContext {
		return &QueryContext{
			ID:               "abc123",
			CustomAddress:    address,
			ServicePortLabel: fmt.Sprintf("%d", port),
			Networks:         nil,
			NetworkStatus:    mock.NewNetworkStatus(address),
			Ports:            nil,
			Group:            "group",
			Task:             "task",
			Service:          "service",
			Check:            "check",
		}
	}

	makeQuery := func(
		kind structs.CheckMode,
		port int,
	) *Query {
		return &Query{
			Mode:        kind,
			Type:        "tcp",
			Timeout:     100 * time.Millisecond,
			AddressMode: "auto",
			PortLabel:   fmt.Sprintf("%d", port),
		}
	}

	makeExpResult := func(
		kind structs.CheckMode,
		status structs.CheckStatus,
		output string,
	) *structs.CheckQueryResult {
		return &structs.CheckQueryResult{
			ID:        "abc123",
			Mode:      kind,
			Status:    status,
			Output:    output,
			Timestamp: now.Unix(),
			Group:     "group",
			Task:      "task",
			Service:   "service",
			Check:     "check",
		}
	}

	ports := freeport.MustTake(3)
	defer freeport.Return(ports)

	cases := []struct {
		name      string
		qc        *QueryContext
		q         *Query
		tcpMode   string // "ok", "off", "hang"
		tcpPort   int
		expResult *structs.CheckQueryResult
	}{{
		name:    "tcp ok",
		qc:      makeQueryContext("localhost", ports[0]),
		q:       makeQuery(structs.Healthiness, ports[0]),
		tcpMode: "ok",
		tcpPort: ports[0],
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckSuccess,
			"nomad: tcp ok",
		),
	}, {
		name:    "tcp not listening",
		qc:      makeQueryContext("127.0.0.1", ports[1]),
		q:       makeQuery(structs.Healthiness, ports[1]),
		tcpMode: "off",
		tcpPort: ports[1],
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckFailure,
			fmt.Sprintf("dial tcp 127.0.0.1:%d: connect: connection refused", ports[1]),
		),
	}, {
		name:    "tcp slow accept",
		qc:      makeQueryContext("localhost", ports[2]),
		q:       makeQuery(structs.Healthiness, ports[2]),
		tcpMode: "hang",
		tcpPort: ports[2],
		expResult: makeExpResult(
			structs.Healthiness,
			structs.CheckFailure,
			"dial tcp: lookup localhost: i/o timeout",
		),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testlog.HCLogger(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c := New(logger)
			c.(*checker).clock = clock

			switch tc.tcpMode {
			case "ok":
				// simulate tcp server by listening
				go tcpServer(ctx, tc.tcpPort)
			case "hang":
				// simulate tcp hang by setting an already expired context
				timeout, stop := context.WithDeadline(ctx, now.Add(-1*time.Second))
				defer stop()
				ctx = timeout
			case "off":
				// simulate tcp dead connection by not listening
			}

			result := c.Do(ctx, tc.qc, tc.q)
			must.Eq(t, tc.expResult, result)
		})
	}
}

func tcpServer(ctx context.Context, port int) {
	var lc net.ListenConfig
	l, _ := lc.Listen(ctx, "tcp", net.JoinHostPort(
		"localhost", fmt.Sprintf("%d", port),
	))
	defer func() {
		_ = l.Close()
	}()
	con, _ := l.Accept()
	defer func() {
		_ = con.Close()
	}()
}

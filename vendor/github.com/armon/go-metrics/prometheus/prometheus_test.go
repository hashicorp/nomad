package prometheus

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	dto "github.com/prometheus/client_model/go"

	"github.com/armon/go-metrics"
	"github.com/prometheus/common/expfmt"
)

const (
	TestHostname = "test_hostname"
)

func MockGetHostname() string {
	return TestHostname
}

func fakeServer(q chan string) *httptest.Server {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		w.Header().Set("Content-Type", "application/json")
		defer r.Body.Close()
		dec := expfmt.NewDecoder(r.Body, expfmt.FmtProtoDelim)
		m := &dto.MetricFamily{}
		dec.Decode(m)
		expectedm := &dto.MetricFamily{
			Name: proto.String("default_one_two"),
			Help: proto.String("default_one_two"),
			Type: dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{
				&dto.Metric{
					Label: []*dto.LabelPair{
						&dto.LabelPair{
							Name:  proto.String("host"),
							Value: proto.String(MockGetHostname()),
						},
					},
					Gauge: &dto.Gauge{
						Value: proto.Float64(42),
					},
				},
			},
		}
		if !reflect.DeepEqual(m, expectedm) {
			msg := fmt.Sprintf("Unexpected samples extracted, got: %+v, want: %+v", m, expectedm)
			q <- errors.New(msg).Error()
		} else {
			q <- "ok"
		}
	}

	return httptest.NewServer(http.HandlerFunc(handler))
}

func TestSetGauge(t *testing.T) {
	q := make(chan string)
	server := fakeServer(q)
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		log.Fatal(err)
	}
	host := u.Hostname() + ":" + u.Port()
	sink, err := NewPrometheusPushSink(host, time.Second, "pushtest")
	metricsConf := metrics.DefaultConfig("default")
	metricsConf.HostName = MockGetHostname()
	metricsConf.EnableHostnameLabel = true
	metrics.NewGlobal(metricsConf, sink)
	metrics.SetGauge([]string{"one", "two"}, 42)
	response := <-q
	if response != "ok" {
		t.Fatal(response)
	}
}

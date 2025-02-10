// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

type mockConsulServer struct {
	httpSrv *httptest.Server

	lock                 sync.RWMutex
	errorCodeOnTokenSelf int
	countTokenSelf       int
}

func (m *mockConsulServer) resetTokenSelf(errNo int) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.countTokenSelf = 0
	m.errorCodeOnTokenSelf = errNo
}

func newMockConsulServer() *mockConsulServer {

	srv := &mockConsulServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/acl/token/self", func(w http.ResponseWriter, r *http.Request) {

		srv.lock.RLock()
		defer srv.lock.RUnlock()
		srv.countTokenSelf++

		if srv.errorCodeOnTokenSelf == 0 {
			secretID := r.Header.Get("X-Consul-Token")
			token := &consulapi.ACLToken{
				SecretID: secretID,
			}
			buf, _ := json.Marshal(token)
			fmt.Fprintf(w, string(buf))
			return
		}

		w.WriteHeader(srv.errorCodeOnTokenSelf)
		fmt.Fprintf(w, "{}")
	})

	srv.httpSrv = httptest.NewServer(mux)
	return srv
}

type testClientCfg struct{ node *structs.Node }

func (c *testClientCfg) GetNode() *structs.Node {
	return c.node
}

// TestConsul_TokenPreflightCheck verifies the retry logic for
func TestConsul_TokenPreflightCheck(t *testing.T) {

	consulSrv := newMockConsulServer()
	consulSrv.resetTokenSelf(404)

	node := mock.Node()
	node.Meta["consul.token_preflight_check.timeout"] = "100ms"
	node.Meta["consul.token_preflight_check.base"] = "10ms"
	clientCfg := &testClientCfg{node}

	factory := NewConsulClientFactory(clientCfg)

	cfg := &config.ConsulConfig{
		Addr: consulSrv.httpSrv.URL,
	}
	client, err := factory(cfg, testlog.HCLogger(t))
	must.NoError(t, err)

	token := &consulapi.ACLToken{
		SecretID:  uuid.Generate(),
		Namespace: "foo",
	}

	preflightErrorCh := make(chan error)

	ctx1, cancel1 := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel1()

	go func() {
		preflightErrorCh <- client.TokenPreflightCheck(ctx1, token)
	}()

	select {
	case <-ctx1.Done():
		t.Fatal("test timed out before check timed out")
	case err := <-preflightErrorCh:
		must.EqError(t, err, "Unexpected response code: 404 ({})")
		must.GreaterEq(t, 5, consulSrv.countTokenSelf)
	}

	consulSrv.resetTokenSelf(0)
	ctx2, cancel2 := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel2()

	go func() {
		preflightErrorCh <- client.TokenPreflightCheck(ctx2, token)
	}()

	select {
	case <-ctx2.Done():
		t.Fatal("test timed out and check should not have timed out")
	case err := <-preflightErrorCh:
		must.NoError(t, err, must.Sprintf("preflight should pass: %v", err))
		must.Eq(t, 1, consulSrv.countTokenSelf)
	}
}

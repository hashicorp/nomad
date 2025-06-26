// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/bufconndialer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestSecretsHook_Prestart_Nomad(t *testing.T) {
	ci.Parallel(t)

	secretsResp := `
	{
	  "CreateIndex": 812,
	  "CreateTime": 1750782609539170600,
	  "Items": {
	    "key2": "value2",
	    "key1": "value1"
	  },
	  "ModifyIndex": 812,
	  "ModifyTime": 1750782609539170600,
	  "Namespace": "default",
	  "Path": "testnomadvar"
	}
	`
	count := 0 // CT expects a nomad index header that incremements, or else it continues polling
	nomadServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Nomad-Index", strconv.Itoa(count))
		fmt.Fprintln(w, secretsResp)
		count += 1
	}))
	t.Cleanup(nomadServer.Close)

	l, d := bufconndialer.New()
	nomadServer.Listener = l

	nomadServer.Start()

	clientConfig := config.DefaultConfig()
	clientConfig.TemplateDialer = d
	clientConfig.TemplateConfig.DisableSandbox = true

	taskDir := t.TempDir()
	alloc := mock.MinAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	conf := &secretsHookConfig{
		logger:       testlog.HCLogger(t),
		lifecycle:    trtesting.NewMockTaskHooks(),
		events:       &trtesting.MockEmitter{},
		clientConfig: clientConfig,
		envBuilder:   taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region),
	}
	secretHook := newSecretsHook(conf, []*structs.Secret{
		{
			Name:     "test_secret",
			Provider: "nomad",
			Path:     "testnomadvar",
			Config: map[string]any{
				"namespace": "default",
			},
		},
	})

	req := &interfaces.TaskPrestartRequest{
		Alloc:   alloc,
		Task:    task,
		TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	err := secretHook.Prestart(ctx, req, nil)
	must.NoError(t, err)

	expected := map[string]string{
		"secret.test_secret.key1": "value1",
		"secret.test_secret.key2": "value2",
	}
	must.Eq(t, expected, secretHook.taskSecrets)
}

func TestSecretsHook_Prestart_Cancelled(t *testing.T) {
	ci.Parallel(t)

	secretsResp := `
	{
	  "CreateIndex": 812,
	  "CreateTime": 1750782609539170600,
	  "Items": {
	    "key2": "value2",
	    "key1": "value1"
	  },
	  "ModifyIndex": 812,
	  "ModifyTime": 1750782609539170600,
	  "Namespace": "default",
	  "Path": "testnomadvar"
	}
	`
	count := 0 // CT expects a nomad index header that incremements, or else it continues polling
	nomadServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Nomad-Index", strconv.Itoa(count))
		fmt.Fprintln(w, secretsResp)
		count += 1
	}))
	t.Cleanup(nomadServer.Close)

	l, d := bufconndialer.New()
	nomadServer.Listener = l

	nomadServer.Start()

	clientConfig := config.DefaultConfig()
	clientConfig.TemplateDialer = d
	clientConfig.TemplateConfig.DisableSandbox = true

	taskDir := t.TempDir()
	alloc := mock.MinAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	conf := &secretsHookConfig{

		logger:       testlog.HCLogger(t),
		lifecycle:    trtesting.NewMockTaskHooks(),
		events:       &trtesting.MockEmitter{},
		clientConfig: clientConfig,
		envBuilder:   taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region),
	}
	secretHook := newSecretsHook(conf, []*structs.Secret{
		{
			Name:     "test_secret",
			Provider: "nomad",
			Path:     "testnomadvar",
			Config: map[string]any{
				"namespace": "default",
			},
		},
	})

	req := &interfaces.TaskPrestartRequest{
		Alloc:   alloc,
		Task:    task,
		TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	cancel() // cancel context to simulate task being stopped

	err := secretHook.Prestart(ctx, req, nil)
	must.NoError(t, err)

	expected := map[string]string{}
	must.Eq(t, expected, secretHook.taskSecrets)
}

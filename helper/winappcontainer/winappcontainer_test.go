// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package winappcontainer

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/winexec"
	"github.com/shoenig/test/must"
)

// TestAppContainer_CatStdin runs a "cat"-like command in an AppContainer and
// pipes data into stdin. We use TestCatHelper to do this so that we don't need
// to rely on external programs
func TestAppContainer_CatStdin(t *testing.T) {
	ci.Parallel(t)
	t.Helper()

	path, _ := os.Executable()

	containerCfg := &AppContainerConfig{
		Name:         t.Name(),
		AllowedPaths: []string{path},
	}
	logger := testlog.HCLogger(t)
	err := CreateAppContainer(logger, containerCfg)
	if err != nil {
		// if the tests are running as an unprivileged user, we might not be
		// able to create the sandbox, but in that case we're not vulnerable to
		// the attacks this is intended to prevent anyways
		must.EqError(t, err, ErrAccessDeniedToCreateSandbox.Error())
	}

	t.Cleanup(func() {
		must.NoError(t, DeleteAppContainer(logger, t.Name()))
	})

	procThreadAttrs, cleanup, err := CreateProcThreadAttributes(t.Name())
	must.NoError(t, err)
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	args := []string{"-test.run=TestCatHelper", "--"}
	cmd := winexec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	cmd.ProcThreadAttributes = procThreadAttrs

	input := "Input string\nLine 2"
	stdin, _ := cmd.StdinPipe()
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, input)
	}()

	bs, err := cmd.CombinedOutput()
	must.EqError(t, err, "exit status 7")
	must.Eq(t, input, string(bs))
}

func TestCatHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("this should only be run as part of the tests above")
		return
	}
	io.Copy(os.Stdout, os.Stdin)
	os.Exit(7)
}

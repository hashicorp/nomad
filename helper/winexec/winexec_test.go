// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package winexec

import (
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

type execCmd interface {
	StdinPipe() (io.WriteCloser, error)
	CombinedOutput() ([]byte, error)
}

// TestWinExec_CatStdin runs a "cat"-like command and pipes data into stdin. We
// use TestCatHelper to do this so that we don't need to rely on external
// programs
func TestWinExec_CatStdin(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name    string
		factory func(context.Context, string, ...string) execCmd
	}{
		{
			name: "winexec.CommandContext",
			factory: func(ctx context.Context, name string, args ...string) execCmd {
				cmd := CommandContext(ctx, name, args...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				return cmd
			},
		},
		{
			// run the exact same test as above, using os/exec's version, so
			// that we can verify we have the exact same behavior
			name: "os/exec.CommandContext",
			factory: func(ctx context.Context, name string, args ...string) execCmd {
				cmd := exec.CommandContext(ctx, name, args...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				return cmd
			},
		},
	}

	for _, tc := range testCases {
		path, _ := os.Executable()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		args := []string{"-test.run=TestCatHelper", "--"}
		cmd := tc.factory(ctx, path, args...)

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
}

func TestCatHelper(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("this should only be run as part of the tests above")
		return
	}
	io.Copy(os.Stdout, os.Stdin)
	os.Exit(7)
}

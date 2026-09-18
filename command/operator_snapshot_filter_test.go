// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestOperatorSnapshotFilter(t *testing.T) {

	snapshotPath := generateSnapshotFile(t, func(srv *agent.TestAgent, client *api.Client, url string) {
		jobs := []string{
			`job "test-job0" {
  type = "service"
  group "g" {
    task "t" {
      driver = "raw_exec"
    }
  }
}`,
			`job "test-job1" {
  type = "system"
  group "g" {
    task "t" {
      driver = "raw_exec"
    }
  }
}`,
		}
		for _, job := range jobs {
			ui := cli.NewMockUi()
			cmd := &JobRunCommand{Meta: Meta{Ui: ui}}
			cmd.JobGetter.testStdin = strings.NewReader(job)
			code := cmd.Run([]string{"--address=" + url, "-detach", "-"})
			must.Zero(t, code)
		}
	})

	runTest := func(t *testing.T, args []string) string {
		t.Helper()

		// the filter command mutates the snapshot in place, so make a quick and
		// dirty copy to run tests on
		outPath := snapshotPath + ".tmp"
		in, _ := os.Open(snapshotPath)
		t.Cleanup(func() { in.Close() })
		out, _ := os.Create(outPath)
		io.Copy(out, in)
		out.Sync()
		out.Close()
		t.Cleanup(func() { os.Remove(outPath) })

		ui := cli.NewMockUi()
		filterCmd := &OperatorSnapshotFilterCommand{Meta: Meta{Ui: ui}}
		args = append(args, outPath)
		code := filterCmd.Run(args)
		must.Eq(t, "", ui.ErrorWriter.String())
		must.Zero(t, code)
		must.StrContains(t, ui.OutputWriter.String(), "Snapshot filtered")

		ui = cli.NewMockUi()
		var buf bytes.Buffer
		showCmd := &OperatorSnapshotStateCommand{Meta: Meta{Ui: ui}, writer: &buf}
		code = showCmd.Run([]string{outPath})
		must.Zero(t, code)
		must.Eq(t, "", ui.ErrorWriter.String())
		return buf.String()
	}

	t.Run("include alone", func(t *testing.T) {
		out := runTest(t, []string{
			"-include", `Type == "system"`, // includes both Jobs and Evals
		})
		test.StrContains(t, out, `"RootKeys": []`,
			test.Sprint("expected RootKeys not to be included"))
		test.StrNotContains(t, out, `"Evals": []`,
			test.Sprint("expected an Evaluation to be included"))

		test.StrContains(t, out, `"ID": "test-job1",`,
			test.Sprint("expected test-job1 to be included"))
		test.StrNotContains(t, out, `"ID": "test-job0",`,
			test.Sprint("expected test-job0 not to be included"))
	})

	t.Run("exclude alone", func(t *testing.T) {
		out := runTest(t, []string{
			"-exclude-types", "3", // excludes Evals
		})
		test.StrNotContains(t, out, `"RootKeys": []`,
			test.Sprint("expected RootKeys not to be excluded by type"))
		test.StrContains(t, out, `"Evals": []`,
			test.Sprint("expected Evaluations to be excluded by type"))

		test.StrContains(t, out, `"ID": "test-job1",`,
			test.Sprint("expected test-job1 not to be excluded by type"))
		test.StrContains(t, out, `"ID": "test-job0",`,
			test.Sprint("expected test-job0 not to be excluded by type"))
	})

	t.Run("include with exclude", func(t *testing.T) {
		out := runTest(t, []string{
			"-include", `Type == "system"`, // includes both Jobs and Evals
			"-exclude-types", "3", // but excludes Evals
		})
		test.StrContains(t, out, `"RootKeys": []`,
			test.Sprint("expected RootKeys not to be included"))
		test.StrContains(t, out, `"Evals": []`,
			test.Sprint("expected Evaluations to be excluded by type"))

		test.StrContains(t, out, `"ID": "test-job1",`,
			test.Sprint("expected test-job1 to be included"))
		test.StrNotContains(t, out, `"ID": "test-job0",`,
			test.Sprint("expected test-job0 not to be included"))
	})

	t.Run("bad type args", func(t *testing.T) {
		ui := cli.NewMockUi()
		filterCmd := &OperatorSnapshotFilterCommand{Meta: Meta{Ui: ui}}
		code := filterCmd.Run([]string{
			"-exclude-types", "a,b",
			snapshotPath,
		})
		must.Eq(t, 1, code)
		must.StrContains(t, ui.ErrorWriter.String(),
			`Could not convert "a" into a numeric type ID`)
	})
}

package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/nomad/api"
)

func TestStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &StatusCommand{}
}

func TestStatusCommand_Run(t *testing.T) {
	srv, client, url := testServer(t, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui}}

	// Should return blank for no jobs
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Check for this awkward nil string, since a nil bytes.Buffer
	// returns this purposely, and mitchellh/cli has a nil pointer
	// if nothing was ever output.
	exp := "No running jobs"
	if out := strings.TrimSpace(ui.OutputWriter.String()); out != exp {
		t.Fatalf("expected %q; got: %q", exp, out)
	}

	// Register two jobs
	job1 := testJob("job1_sfx")
	evalId1, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, evalId1); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	job2 := testJob("job2_sfx")
	evalId2, _, err := client.Jobs().Register(job2, nil);
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, evalId2); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query again and check the result
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected job1_sfx and job2_sfx, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query a single job
	if code := cmd.Run([]string{"-address=" + url, "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	if !strings.Contains(out, "Summary") {
		t.Fatalf("should dump summary")
	}
	ui.OutputWriter.Reset()

	// Query a single job showing evals
	if code := cmd.Run([]string{"-address=" + url, "-evals", "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Evaluations") {
		t.Fatalf("should dump evaluations")
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	ui.OutputWriter.Reset()

	// Query a single job in verbose mode
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "job2_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job2_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Evaluations") {
		t.Fatalf("should dump evaluations")
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("should dump allocations")
	}
	if strings.Contains(out, "Created") {
		t.Fatal("should not have created header")
	}
	ui.OutputWriter.Reset()

	// Query a single job in time mode
	if code := cmd.Run([]string{"-address=" + url, "-time", "job1_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if strings.Contains(out, "job2_sfx") || !strings.Contains(out, "job1_sfx") {
		t.Fatalf("expected only job1_sfx, got: %s", out)
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatal("should dump allocations")
	}
	if !strings.Contains(out, "Created") {
		t.Fatal("should have created header")
	}
	ui.OutputWriter.Reset()

	// Query jobs with prefix match
	if code := cmd.Run([]string{"-address=" + url, "job"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "job1_sfx") || !strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected job1_sfx and job2_sfx, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query a single job with prefix match
	if code := cmd.Run([]string{"-address=" + url, "job1"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "job1_sfx") || strings.Contains(out, "job2_sfx") {
		t.Fatalf("expected only job1_sfx, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query in short view mode
	if code := cmd.Run([]string{"-address=" + url, "-short", "job2"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "job2") {
		t.Fatalf("expected job2, got: %s", out)
	}
	if strings.Contains(out, "Evaluations") {
		t.Fatalf("should not dump evaluations")
	}
	if strings.Contains(out, "Allocations") {
		t.Fatalf("should not dump allocations")
	}
	if strings.Contains(out, evalId1) {
		t.Fatalf("should not contain full identifiers, got %s", out)
	}
	ui.OutputWriter.Reset()

	// Request full identifiers
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "job1"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, evalId1) {
		t.Fatalf("should contain full identifiers, got %s", out)
	}
}

func TestStatusCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying jobs") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}

func waitForSuccess(ui cli.Ui, client *api.Client, length int, t *testing.T, evalId string) int {
	mon := newMonitor(ui, client, length)
	monErr := mon.monitor(evalId, false)
	return monErr
}
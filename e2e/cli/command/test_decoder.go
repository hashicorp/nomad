package command

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	hclog "github.com/hashicorp/go-hclog"
)

type EventDecoder struct {
	r io.Reader

	dec    *json.Decoder
	report *TestReport
}

type TestReport struct {
	Events            []*TestEvent
	Suites            map[string]*TestSuite
	TotalSuites       int
	TotalFailedSuites int
	TotalCases        int
	TotalFailedCases  int
	TotalTests        int
	TotalFailedTests  int
	Elapsed           float64
	Output            []string
}

type TestEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string

	suiteName string
	caseName  string
	testName  string
}

type TestSuite struct {
	Name    string
	Cases   map[string]*TestCase
	Failed  int
	Elapsed float64
	Output  []string
}

type TestCase struct {
	Name    string
	Tests   map[string]*Test
	Failed  int
	Elapsed float64
	Output  []string
}

type Test struct {
	Name    string
	Output  []string
	Failed  bool
	Elapsed float64
}

func NewDecoder(r io.Reader) *EventDecoder {
	return &EventDecoder{
		r:   r,
		dec: json.NewDecoder(r),
		report: &TestReport{
			Suites: map[string]*TestSuite{},
			Events: []*TestEvent{},
		},
	}
}

func (d *EventDecoder) Decode(logger hclog.Logger) (*TestReport, error) {
	for d.dec.More() {
		var e TestEvent
		err := d.dec.Decode(&e)
		if err != nil {
			return nil, err
		}

		d.report.record(&e)
		if logger != nil && e.Output != "" {
			logger.Debug(strings.TrimRight(e.Output, "\n"))
		}
	}
	return d.report, nil
}

func (r *TestReport) record(event *TestEvent) {
	if !strings.HasPrefix(event.Test, "TestE2E") {
		return
	}
	parts := strings.Split(event.Test, "/")
	switch len(parts) {
	case 1:
		r.recordRoot(event)
	case 2:
		event.suiteName = parts[1]
		r.recordSuite(event)
	case 3:
		event.suiteName = parts[1]
		event.caseName = parts[2]
		r.recordCase(event, r.Suites[event.suiteName])
	case 4:
		event.suiteName = parts[1]
		event.caseName = parts[2]
		event.testName = strings.Join(parts[3:], "/")
		suite := r.Suites[event.suiteName]
		r.recordTest(event, suite, suite.Cases[event.caseName])
	}
	r.Events = append(r.Events, event)

}

func (r *TestReport) recordRoot(event *TestEvent) {
	switch event.Action {
	case "run":
	case "output":
		r.Output = append(r.Output, event.Output)
	case "pass", "fail":
		r.Elapsed = event.Elapsed
	}
}
func (r *TestReport) recordSuite(event *TestEvent) {
	switch event.Action {
	case "run":
		r.Suites[event.suiteName] = &TestSuite{
			Name:  event.suiteName,
			Cases: map[string]*TestCase{},
		}
		r.TotalSuites += 1
	case "output":
		r.Suites[event.suiteName].Output = append(r.Suites[event.suiteName].Output, event.Output)
	case "pass":
		r.Suites[event.suiteName].Elapsed = event.Elapsed
	case "fail":
		r.Suites[event.suiteName].Elapsed = event.Elapsed
		r.TotalFailedSuites += 1
	}
}
func (r *TestReport) recordCase(event *TestEvent, suite *TestSuite) {
	switch event.Action {
	case "run":
		suite.Cases[event.caseName] = &TestCase{
			Name:  event.caseName,
			Tests: map[string]*Test{},
		}
		r.TotalCases += 1
	case "output":
		suite.Cases[event.caseName].Output = append(suite.Cases[event.caseName].Output, event.Output)
	case "pass":
		suite.Cases[event.caseName].Elapsed = event.Elapsed
	case "fail":
		suite.Cases[event.caseName].Elapsed = event.Elapsed
		suite.Failed += 1
		r.TotalFailedCases += 1
	}
}
func (r *TestReport) recordTest(event *TestEvent, suite *TestSuite, c *TestCase) {
	switch event.Action {
	case "run":
		c.Tests[event.testName] = &Test{
			Name: event.testName,
		}
		r.TotalTests += 1
	case "output":
		c.Tests[event.testName].Output = append(c.Tests[event.testName].Output, event.Output)
	case "pass":
		c.Tests[event.testName].Elapsed = event.Elapsed
	case "fail":
		c.Tests[event.testName].Elapsed = event.Elapsed
		c.Tests[event.testName].Failed = true
		c.Failed += 1
		r.TotalFailedTests += 1
	}
}

func (r *TestReport) Summary() string {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	sb := strings.Builder{}
	sb.WriteString(
		fmt.Sprintf("Summary:  %v/%v suites failed  |  %v/%v cases failed  |  %v/%v tests failed\n",
			r.TotalFailedSuites, r.TotalSuites,
			r.TotalFailedCases, r.TotalCases,
			r.TotalFailedTests, r.TotalTests))

	sb.WriteString("Details:\n")
	w := tabwriter.NewWriter(&sb, 0, 0, 1, ' ', tabwriter.AlignRight)
	for sname, suite := range r.Suites {
		status := red("FAIL")
		if suite.Failed == 0 {
			status = green("PASS")
		}

		fmt.Fprintf(w, "[%s]\t%s\t\t\t (%vs)\n", status, sname, suite.Elapsed)
		for cname, c := range suite.Cases {
			status := red("FAIL")
			if c.Failed == 0 {
				status = green("PASS")
			}
			fmt.Fprintf(w, "[%s]\t↳\t%s\t\t (%vs)\n", status, cname, c.Elapsed)
			for tname, test := range c.Tests {
				status := red("FAIL")
				if !test.Failed {
					status = green("PASS")
				}
				fmt.Fprintf(w, "[%s]\t\t↳\t%s\t (%vs)\n", status, tname, test.Elapsed)
				if test.Failed {
					for _, line := range test.Output[2:] {
						fmt.Fprintf(w, "\t\t\t%s\n", strings.Replace(strings.TrimSpace(line), "\t", "  ", -1))
					}
					fmt.Fprintln(w, "\t\t\t----------")
				}

			}
		}
	}

	w.Flush()
	return sb.String()
}

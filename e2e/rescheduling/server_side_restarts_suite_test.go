package rescheduling

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var integration = flag.Bool("integration", false, "run integration tests")
var slow = flag.Bool("slow", false, "runs slower integration tests")

func TestServerSideRestarts(t *testing.T) {
	if !*integration {
		t.Skip("skipping test in non-integration mode.")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Side Restart Tests")
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consultemplate

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
)

func TestConsulTemplate(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
		cluster3.Timeout(10*time.Second),
	)

	t.Run("testNomadServices", testNomadServices)
}

func testNomadServices(t *testing.T) {
	// hi
}

package command

import (
	"strings"
	"testing"
)

func TestValidateTLSCertCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(NewCert().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

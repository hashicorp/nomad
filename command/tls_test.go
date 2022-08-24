package command

import (
	"strings"
	"testing"
)

func TestValidateTLSCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(NewTLS().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

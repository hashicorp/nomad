package command

import (
	"strings"
	"testing"
)

func TestValidateTLSCACommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(NewCA().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

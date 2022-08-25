package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestValidateTLSCertCommand_HasTabs(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	cmd := &TLSCertCommand{Meta: Meta{Ui: ui}}
	code := cmd.Help()
	require.False(t, strings.ContainsRune(code, '\t'))
}

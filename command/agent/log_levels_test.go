package agent

import (
	"testing"

	"github.com/hashicorp/logutils"
)

func TestLevelFilter(t *testing.T) {
	t.Parallel()

	filt := LevelFilter()
	filt.Levels = []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"}
	level := logutils.LogLevel("INFO")

	// LevelFilter regards INFO as valid level
	if !ValidateLevelFilter(level, filt) {
		t.Fatalf("expected valid LogLevel, %s was invalid", level)
	}

	level = logutils.LogLevel("FOO")

	// LevelFilter regards FOO as invalid level
	if ValidateLevelFilter(level, filt) {
		t.Fatalf("expected invalid LogLevel, %s was valid", level)
	}

}

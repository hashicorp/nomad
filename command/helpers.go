package command

import (
	"github.com/ryanuber/columnize"
)

// formatKV takes a set of strings and formats them into properly
// aligned k = v pairs using the columnize library.
func formatKV(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	columnConf.Glue = " = "
	return columnize.Format(in, columnConf)
}

// formatList takes a set of strings and formats them into properly
// aligned output, replacing any blank fields with a placeholder
// for awk-ability.
func formatList(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	return columnize.Format(in, columnConf)
}

// Limits the length of the string.
func limit(s string, length int) string {
	if len(s) < length {
		return s[:len(s)]
	}

	return s[:length]
}

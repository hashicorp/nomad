package command

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/api"
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

// formatListWithSpaces takes a set of strings and formats them into properly
// aligned output. It should be used sparingly since it doesn't replace empty
// values and hence not awk/sed friendly
func formatListWithSpaces(in []string) string {
	columnConf := columnize.DefaultConfig()
	return columnize.Format(in, columnConf)
}

// Limits the length of the string.
func limit(s string, length int) string {
	if len(s) < length {
		return s
	}

	return s[:length]
}

// formatTime formats the time to string based on RFC822
func formatTime(t time.Time) string {
	return t.Format("01/02/06 15:04:05 MST")
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func formatTimeDifference(first, second time.Time, d time.Duration) string {
	return second.Truncate(d).Sub(first.Truncate(d)).String()
}

// getLocalNodeID returns the node ID of the local Nomad Client and an error if
// it couldn't be determined or the Agent is not running in Client mode.
func getLocalNodeID(client *api.Client) (string, error) {
	info, err := client.Agent().Self()
	if err != nil {
		return "", fmt.Errorf("Error querying agent info: %s", err)
	}
	var stats map[string]interface{}
	stats, _ = info["stats"]
	clientStats, ok := stats["client"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Nomad not running in client mode")
	}

	nodeID, ok := clientStats["node_id"].(string)
	if !ok {
		return "", fmt.Errorf("Failed to determine node ID")
	}

	return nodeID, nil
}

// evalFailureStatus returns whether the evaluation has failures and a string to
// display when presenting users with whether there are failures for the eval
func evalFailureStatus(eval *api.Evaluation) (string, bool) {
	if eval == nil {
		return "", false
	}

	hasFailures := len(eval.FailedTGAllocs) != 0
	text := strconv.FormatBool(hasFailures)
	if eval.Status == "blocked" {
		text = "N/A - In Progress"
	}

	return text, hasFailures
}

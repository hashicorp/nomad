// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Command sends a command line argument to Nomad and returns the unbuffered
// stdout as a string (or, if there's an error, the stderr)
func Command(cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	bytes, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	out := string(bytes)
	if err != nil {
		return out, fmt.Errorf("command %v %v failed: %v\nOutput: %v", cmd, args, err, out)
	}
	return out, err
}

// GetField returns the value of an output field (ex. the "Submit Date" field
// of `nomad job status :id`)
func GetField(output, key string) (string, error) {
	re := regexp.MustCompile(`(?m)^` + key + ` += (.*)$`)
	match := re.FindStringSubmatch(output)
	if match == nil {
		return "", fmt.Errorf("could not find field %q", key)
	}
	return match[1], nil
}

// GetSection returns a section, with its field header but without its title.
// (ex. the Allocations section of `nomad job status :id`)
func GetSection(output, key string) (string, error) {

	// golang's regex engine doesn't support negative lookahead, so
	// we can't stop at 2 newlines if we also want a section that includes
	// single newlines. so split on the section title, and then split a second time
	// on \n\n
	re := regexp.MustCompile(`(?ms)^` + key + `\n(.*)`)
	match := re.FindStringSubmatch(output)
	if match == nil {
		return "", fmt.Errorf("could not find section %q", key)
	}
	tail := match[1]
	return strings.Split(tail, "\n\n")[0], nil
}

// ParseColumns maps the CLI output for a columized section (without title) to
// a slice of key->value pairs for each row in that section.
// (ex. the Allocations section of `nomad job status :id`)
func ParseColumns(section string) ([]map[string]string, error) {
	parsed := []map[string]string{}

	// field names and values are deliminated by two or more spaces, but can have a
	// single space themselves. compress all the delimiters into a tab so we can
	// break the fields on that
	re := regexp.MustCompile(" {2,}")
	section = re.ReplaceAllString(section, "\t")
	rows := strings.Split(section, "\n")

	breakFields := func(row string) []string {
		return strings.FieldsFunc(row, func(c rune) bool { return c == '\t' })
	}

	fieldNames := breakFields(rows[0])

	for _, row := range rows[1:] {
		if row == "" {
			continue
		}
		r := map[string]string{}
		vals := breakFields(row)
		for i, val := range vals {
			if i >= len(fieldNames) {
				return parsed, fmt.Errorf("section is misaligned with header\n%v", section)
			}

			r[fieldNames[i]] = val
		}
		parsed = append(parsed, r)
	}
	return parsed, nil
}

// ParseFields maps the CLI output for a key-value section (without title) to
// map of the key->value pairs in that section
// (ex. the Latest Deployment section of `nomad job status :id`)
func ParseFields(section string) (map[string]string, error) {
	parsed := map[string]string{}
	rows := strings.Split(strings.TrimSpace(section), "\n")
	for _, row := range rows {
		kv := strings.Split(row, "=")
		if len(kv) == 0 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		if len(kv) == 1 {
			parsed[key] = ""
		} else {
			parsed[key] = strings.TrimSpace(strings.Join(kv[1:], " "))
		}
	}
	return parsed, nil
}

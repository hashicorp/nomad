// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package anonymous

import (
	"fmt"
	"regexp"
	"strconv"
)

// String creates a faux username encoding the given ugid.
func String(ugid UGID) string {
	return fmt.Sprintf("nomad:%d", ugid)
}

var (
	re = regexp.MustCompile(`^nomad:(\d+)$`)
)

// Parse the given faux username and extract the ugid.
func Parse(user string) (UGID, error) {
	values := re.FindStringSubmatch(user)
	if len(values) != 2 {
		return none, ErrCannotParse
	}

	i, err := strconv.ParseUint(values[1], 10, 64)
	if err != nil {
		return none, ErrCannotParse
	}

	return UGID(i), err
}

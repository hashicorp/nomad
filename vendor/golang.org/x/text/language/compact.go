// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package language

import (
	"sort"
	"strings"

	"golang.org/x/text/internal/language"
)

type compactID uint16

func getCoreIndex(t language.Tag) (id compactID, ok bool) {
	cci, ok := language.GetCompactCore(t)
	if !ok {
		return 0, false
	}
	i := sort.Search(len(coreTags), func(i int) bool {
		return cci <= coreTags[i]
	})
	if i == len(coreTags) || coreTags[i] != cci {
		return 0, false
	}
	return compactID(i), true
}

func (c compactID) tag() language.Tag {
	if int(c) >= len(coreTags) {
		return specialTags[int(c)-len(coreTags)]
	}
	return coreTags[c].Tag()
}

var specialTags []language.Tag

func init() {
	tags := strings.Split(specialTagsStr, " ")
	specialTags = make([]language.Tag, len(tags))
	for i, t := range tags {
		specialTags[i] = language.MustParse(t)
	}
}

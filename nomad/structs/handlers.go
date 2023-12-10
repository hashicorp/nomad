// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/hashicorp/go-msgpack/codec"
)

var (
	// JsonHandle and JsonHandlePretty are the codec handles to JSON encode
	// structs. The pretty handle will add indents for easier human consumption.
	// JsonHandleWithExtensions and JsonHandlePretty include extensions for
	// encoding structs objects with API-specific fields
	JsonHandle = &codec.JsonHandle{
		HTMLCharsAsIs: true,
	}
	JsonHandleWithExtensions = NomadJsonEncodingExtensions(&codec.JsonHandle{
		HTMLCharsAsIs: true,
	})
	JsonHandlePretty = NomadJsonEncodingExtensions(&codec.JsonHandle{
		HTMLCharsAsIs: true,
		Indent:        4,
	})
)

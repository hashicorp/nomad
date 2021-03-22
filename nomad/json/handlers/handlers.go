package handlers

import (
	"github.com/hashicorp/go-msgpack/codec"

	"github.com/hashicorp/nomad/nomad/json"
)

var (
	// JsonHandle and JsonHandlePretty are the codec handles to JSON encode
	// structs. The pretty handle will add indents for easier human consumption.
	// JsonHandleWithExtensions and JsonHandlePretty include extensions for
	// encoding structs objects with API-specific fields
	JsonHandle = &codec.JsonHandle{
		HTMLCharsAsIs: true,
	}
	JsonHandleWithExtensions = json.NomadJsonEncodingExtensions(&codec.JsonHandle{
		HTMLCharsAsIs: true,
	})
	JsonHandlePretty = json.NomadJsonEncodingExtensions(&codec.JsonHandle{
		HTMLCharsAsIs: true,
		Indent:        4,
	})
)

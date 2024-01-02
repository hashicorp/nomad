// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/hashicorp/go-msgpack/codec"
)

var (
	jsonHandlePretty = &codec.JsonHandle{
		HTMLCharsAsIs: true,
		Indent:        4,
	}
)

// DataFormatter is a transformer of the data.
type DataFormatter interface {
	// TransformData should return transformed string data.
	TransformData(interface{}) (string, error)
}

// DataFormat returns the data formatter specified format.
func DataFormat(format, tmpl string) (DataFormatter, error) {
	switch format {
	case "json":
		if len(tmpl) > 0 {
			return nil, fmt.Errorf("json format does not support template option.")
		}
		return &JSONFormat{}, nil
	case "template":
		return &TemplateFormat{tmpl}, nil
	}
	return nil, fmt.Errorf("Unsupported format is specified.")
}

type JSONFormat struct{}

// TransformData returns JSON format string data.
func (p *JSONFormat) TransformData(data interface{}) (string, error) {
	var buf bytes.Buffer
	err := codec.NewEncoder(&buf, jsonHandlePretty).Encode(data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

type TemplateFormat struct {
	tmpl string
}

// TransformData returns template format string data.
func (p *TemplateFormat) TransformData(data interface{}) (string, error) {
	var out bytes.Buffer
	if len(p.tmpl) == 0 {
		return "", fmt.Errorf("template needs to be specified in golang's text/template format.")
	}

	t, err := template.New("").Funcs(makeFuncMap()).Parse(p.tmpl)
	if err != nil {
		return "", err
	}

	err = t.Execute(&out, data)
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func Format(json bool, template string, data interface{}) (string, error) {
	var format string
	if json && len(template) > 0 {
		return "", fmt.Errorf("Both json and template formatting are not allowed")
	} else if json {
		format = "json"
	} else if len(template) > 0 {
		format = "template"
	} else {
		return "", fmt.Errorf("no formatting option given")
	}

	f, err := DataFormat(format, template)
	if err != nil {
		return "", err
	}

	out, err := f.TransformData(data)
	if err != nil {
		return "", fmt.Errorf("Error formatting the data: %w", err)
	}

	return out, nil
}

func makeFuncMap() template.FuncMap {
	fm := template.FuncMap{}

	// Add the Sprig functions to the funcmap. These functions are decorated
	// with `sprig_` to match how they are treated in consul-template
	for k, v := range sprig.FuncMap() {
		target := "sprig_" + k
		fm[target] = v
	}

	return fm
}

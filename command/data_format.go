package command

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"github.com/hashicorp/go-msgpack/codec"
)

var (
	jsonHandlePretty = &codec.JsonHandle{
		HTMLCharsAsIs: true,
		Indent:        4,
	}
)

//DataFormatter is a transformer of the data.
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

type JSONFormat struct {
}

// TransformData returns JSON format string data.
func (p *JSONFormat) TransformData(data interface{}) (string, error) {
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, jsonHandlePretty)
	err := enc.Encode(data)
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
	var out io.Writer = new(bytes.Buffer)
	if len(p.tmpl) == 0 {
		return "", fmt.Errorf("template needs to be specified the golang templates.")
	}

	t, err := template.New("format").Parse(p.tmpl)
	if err != nil {
		return "", err
	}

	err = t.Execute(out, data)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(out), nil
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
		return "", fmt.Errorf("Error formatting the data: %s", err)
	}

	return out, nil
}

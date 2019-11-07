package command

import (
	"strings"
	"testing"
)

type testData struct {
	Region string
	ID     string
	Name   string
}

const expectJSON = `{
    "ID": "1",
    "Name": "example",
    "Region": "global"
}`

var (
	tData        = testData{"global", "1", "example"}
	testFormat   = map[string]string{"json": "", "template": "{{.Region}}"}
	expectOutput = map[string]string{"json": expectJSON, "template": "global"}
)

func TestDataFormat(t *testing.T) {
	t.Parallel()
	for k, v := range testFormat {
		fm, err := DataFormat(k, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		result, err := fm.TransformData(tData)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if result != expectOutput[k] {
			t.Fatalf("expected output:\n%s\nactual:\n%s", expectOutput[k], result)
		}
	}
}

func TestInvalidJSONTemplate(t *testing.T) {
	t.Parallel()
	// Invalid template {{.foo}}
	fm, err := DataFormat("template", "{{.foo}}")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = fm.TransformData(tData)
	if !strings.Contains(err.Error(), "can't evaluate field foo") {
		t.Fatalf("expected invalid template error, got: %s", err.Error())
	}

	// No template is specified
	fm, err = DataFormat("template", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = fm.TransformData(tData)
	if !strings.Contains(err.Error(), "template needs to be specified the golang templates.") {
		t.Fatalf("expected not specified template error, got: %s", err.Error())
	}
}

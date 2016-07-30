package command

import (
	"testing"
)

type testData struct {
	Region string
	ID     string
	Name   string
}

const expectJSON = `{
    "Region": "global",
    "ID": "1",
    "Name": "example"
}`

var (
	tData        = testData{"global", "1", "example"}
	testFormat   = map[string]string{"json": "", "template": "{{.Region}}"}
	expectOutput = map[string]string{"json": expectJSON, "template": "global"}
)

func TestDataFormat(t *testing.T) {
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
			t.Fatalf("expected output: %s, actual: %s", expectOutput[k], result)
		}
	}
}

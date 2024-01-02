// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ui

import (
	"bytes"
	"io"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestLogUI_Implements(t *testing.T) {
	var _ cli.Ui = new(LogUI)
}

func TestLogUI_Ask(t *testing.T) {
	testCases := []struct {
		name           string
		query          string
		input          string
		expectedQuery  string
		expectedResult string
	}{
		{
			name:           "EmptyString",
			query:          "Middle Name?",
			input:          "\n",
			expectedQuery:  "Middle Name? ",
			expectedResult: "",
		},
		{
			name:           "NonEmptyString",
			query:          "Name?",
			input:          "foo bar\nbaz\n",
			expectedQuery:  "Name? ",
			expectedResult: "foo bar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inReader, inWriter := io.Pipe()
			defer inReader.Close()
			defer inWriter.Close()

			writer := new(bytes.Buffer)

			logUI, err := NewLogUI(&cli.BasicUi{
				Reader: inReader,
				Writer: writer,
			})
			must.NoError(t, err)

			go inWriter.Write([]byte(tc.input))

			result, err := logUI.Ask(tc.query)
			must.NoError(t, err)
			must.Eq(t, writer.String(), tc.expectedQuery)
			must.Eq(t, result, tc.expectedResult)
		})
	}
}

func TestLogUI_AskSecret(t *testing.T) {
	inReader, inWriter := io.Pipe()
	defer inReader.Close()
	defer inWriter.Close()

	writer := new(bytes.Buffer)
	logUI, err := NewLogUI(&cli.BasicUi{
		Reader: inReader,
		Writer: writer,
	})
	must.NoError(t, err)

	go inWriter.Write([]byte("foo bar\nbaz\n"))

	result, err := logUI.AskSecret("Name?")
	must.NoError(t, err)
	must.Eq(t, writer.String(), "Name? ")
	must.Eq(t, result, "foo bar")
}

func TestLogUI_Error(t *testing.T) {
	writer := new(bytes.Buffer)
	logUI, err := NewLogUI(&cli.BasicUi{Writer: writer})
	must.NoError(t, err)
	logUI.Error("ERROR")
	must.Eq(t, writer.String(), "ERROR")

	writer = new(bytes.Buffer)
	logUI, err = NewLogUI(&cli.ColoredUi{Ui: &cli.BasicUi{Writer: writer}})
	must.NoError(t, err)
	logUI.Error("ERROR")
	must.Eq(t, writer.String(), "ERROR")
}

func TestLogUI_Output(t *testing.T) {
	writer := new(bytes.Buffer)
	logUI, err := NewLogUI(&cli.BasicUi{Writer: writer})
	must.NoError(t, err)
	logUI.Error("OUTPUT")
	must.Eq(t, writer.String(), "OUTPUT")

	writer = new(bytes.Buffer)
	logUI, err = NewLogUI(&cli.ColoredUi{Ui: &cli.BasicUi{Writer: writer}})
	must.NoError(t, err)
	logUI.Error("OUTPUT")
	must.Eq(t, writer.String(), "OUTPUT")
}

func TestLogUI_Warn(t *testing.T) {
	writer := new(bytes.Buffer)
	logUI, err := NewLogUI(&cli.BasicUi{Writer: writer})
	must.NoError(t, err)
	logUI.Error("WARN")
	must.Eq(t, writer.String(), "WARN")

	writer = new(bytes.Buffer)
	logUI, err = NewLogUI(&cli.ColoredUi{Ui: &cli.BasicUi{Writer: writer}})
	must.NoError(t, err)
	logUI.Error("WARN")
	must.Eq(t, writer.String(), "WARN")
}

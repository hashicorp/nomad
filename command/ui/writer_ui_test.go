// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ui

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestWriterUI_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Ui = new(WriterUI)
}

type writerUITestCase struct {
	name   string                  // the testcase name
	baseUi cli.Ui                  // cli.Ui with accessible writers (currently basicUi or mockUi)
	ui     cli.Ui                  // the full ui object chain (should end in baseUi)
	initFn func(*writerUITestCase) // sets up the struct for the testcase
	ow     *bytes.Buffer           // handle to basicUi's Output writer
	ew     *bytes.Buffer           // handle to basicUi's Error writer
}

func TestWriterUI_OutputWriter(t *testing.T) {
	ci.Parallel(t)

	tcs := []writerUITestCase{
		{
			name: "mockUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = tc.baseUi
			},
		},
		{
			name: "mockUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "mockUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
		{
			name: "basicUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = tc.baseUi
			},
		},
		{
			name: "basicUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "basicUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc.initFn(&tc)

			wUI, err := NewWriterUI(tc.ui)
			must.NoError(t, err)
			fmt.Fprintf(wUI.OutputWriter(), "foobar")

			switch bui := tc.baseUi.(type) {
			case *cli.MockUi:
				must.Eq(t, "foobar", bui.OutputWriter.String())
				must.Eq(t, "", bui.ErrorWriter.String())
			case *cli.BasicUi:
				must.Eq(t, "foobar", tc.ow.String())
				must.Eq(t, "", tc.ew.String())
			default:
				t.Fatal("invalid base cli.Ui type")
			}
		})
	}
}

func TestWriterUI_Output(t *testing.T) {
	ci.Parallel(t)

	tcs := []writerUITestCase{
		{
			name: "mockUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = tc.baseUi
			},
		},
		{
			name: "mockUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "mockUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
		{
			name: "basicUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = tc.baseUi
			},
		},
		{
			name: "basicUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "basicUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc.initFn(&tc)

			wUI, err := NewWriterUI(tc.ui)
			must.NoError(t, err)
			wUI.Output("foobar")

			var ov, ev string
			switch bui := tc.baseUi.(type) {
			case *cli.MockUi:
				ov = bui.OutputWriter.String()
				ev = bui.ErrorWriter.String()
			case *cli.BasicUi:
				ov = tc.ow.String()
				ev = tc.ew.String()
			default:
				t.Fatal("invalid base cli.Ui type")
			}

			must.Eq(t, "foobar\n", ov)
			must.Eq(t, "", ev)
		})
	}
}

func TestWriterUI_Info(t *testing.T) {
	ci.Parallel(t)

	tcs := []writerUITestCase{
		{
			name: "mockUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = tc.baseUi
			},
		},
		{
			name: "mockUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "mockUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
		{
			name: "basicUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = tc.baseUi
			},
		},
		{
			name: "basicUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "basicUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc.initFn(&tc)

			wUI, err := NewWriterUI(tc.ui)
			must.NoError(t, err)
			wUI.Info("INFO")

			var ov, ev string
			switch bui := tc.baseUi.(type) {
			case *cli.MockUi:
				ov = bui.OutputWriter.String()
				ev = bui.ErrorWriter.String()
			case *cli.BasicUi:
				ov = tc.ow.String()
				ev = tc.ew.String()
			default:
				t.Fatal("invalid base cli.Ui type")
			}

			must.Eq(t, "INFO\n", ov)
			must.Eq(t, "", ev)
		})
	}
}

func TestWriterUI_Warn(t *testing.T) {
	ci.Parallel(t)

	tcs := []writerUITestCase{
		{
			name: "mockUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = tc.baseUi
			},
		},
		{
			name: "mockUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "mockUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
		{
			name: "basicUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = tc.baseUi
			},
		},
		{
			name: "basicUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "basicUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc.initFn(&tc)

			wUI, err := NewWriterUI(tc.ui)
			must.NoError(t, err)
			wUI.Warn("WARN")

			const expected = "WARN\n"

			var ov, ev string
			switch bui := tc.baseUi.(type) {
			case *cli.MockUi:
				ov = bui.OutputWriter.String()
				ev = bui.ErrorWriter.String()
			case *cli.BasicUi:
				ov = tc.ow.String()
				ev = tc.ew.String()
			default:
				t.Fatal("invalid base cli.Ui type")
			}

			must.Eq(t, "", ov)
			must.Eq(t, "WARN\n", ev)
		})
	}
}

func TestWriterUI_Error(t *testing.T) {
	ci.Parallel(t)

	tcs := []writerUITestCase{
		{
			name: "mockUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = tc.baseUi
			},
		},
		{
			name: "mockUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "mockUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.baseUi = cli.NewMockUi()
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
		{
			name: "basicUi/simple",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = tc.baseUi
			},
		},
		{
			name: "basicUi/nested_once",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: tc.baseUi}
			},
		},
		{
			name: "basicUi/nested_twice",
			initFn: func(tc *writerUITestCase) {
				tc.ow = new(bytes.Buffer)
				tc.ew = new(bytes.Buffer)
				tc.baseUi = &cli.BasicUi{Writer: tc.ow, ErrorWriter: tc.ew}
				tc.ui = &cli.ColoredUi{Ui: &cli.ColoredUi{Ui: tc.baseUi}}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc.initFn(&tc)

			wUI, err := NewWriterUI(tc.ui)
			must.NoError(t, err)
			wUI.Warn("ERROR")

			var ov, ev string
			switch bui := tc.baseUi.(type) {
			case *cli.MockUi:
				ov = bui.OutputWriter.String()
				ev = bui.ErrorWriter.String()
			case *cli.BasicUi:
				ov = tc.ow.String()
				ev = tc.ew.String()
			default:
				t.Fatal("invalid base cli.Ui type")
			}

			must.Eq(t, "", ov)
			must.Eq(t, "ERROR\n", ev)
		})
	}
}

func TestWriterUI_Ask(t *testing.T) {
	ci.Parallel(t)

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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			inReader, inWriter := io.Pipe()
			t.Cleanup(func() {
				inReader.Close()
				inWriter.Close()
			})

			writer := new(bytes.Buffer)

			WriterUI, err := NewWriterUI(&cli.BasicUi{
				Reader: inReader,
				Writer: writer,
			})
			must.NoError(t, err)

			go inWriter.Write([]byte(tc.input))

			result, err := WriterUI.Ask(tc.query)
			must.NoError(t, err)
			must.Eq(t, writer.String(), tc.expectedQuery)
			must.Eq(t, result, tc.expectedResult)
		})
	}
}

func TestWriterUI_AskSecret(t *testing.T) {
	ci.Parallel(t)

	inReader, inWriter := io.Pipe()
	t.Cleanup(func() {
		inReader.Close()
		inWriter.Close()
	})

	writer := new(bytes.Buffer)
	wUI, err := NewWriterUI(&cli.BasicUi{
		Reader: inReader,
		Writer: writer,
	})
	must.NoError(t, err)

	go inWriter.Write([]byte("foo bar\nbaz\n"))

	result, err := wUI.AskSecret("Name?")
	must.NoError(t, err)
	must.Eq(t, writer.String(), "Name? ")
	must.Eq(t, result, "foo bar")
}

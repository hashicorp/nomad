package main

import (
	"io"
	"os"
	"os/exec"
	"text/template"
)

// open the output file for writing.
func open(output string) (io.ReadWriteCloser, error) {
	return os.Create(output)
}

// flatten region data, assuming instance type is the same across regions.
func flatten(data map[string]map[string]specs) map[string]specs {
	result := make(map[string]specs)
	for _, m := range data {
		for iType, specs := range m {
			result[iType] = specs
		}
	}
	return result
}

type Template struct {
	Package string
	Data    map[string]specs
}

// write the data using the cpu_table.go.template to w.
func write(w io.Writer, data map[string]specs, pkg string) error {
	tmpl, err := template.ParseFiles("cpu_table.go.template")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, Template{
		Package: pkg,
		Data:    data,
	})
}

// format the file using gofmt.
func format(file string) error {
	cmd := exec.Command("gofmt", "-w", file)
	_, err := cmd.CombinedOutput()
	return err
}

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_Equals(t *testing.T) {
	req := require.New(t)

	g := &Generator{
		typeNames:  []string{"Service"},
		packageDir: "../../nomad/structs",
		methods:    []string{"Service.Equals"},
		typeSpecs:  map[string]*TypeSpecNode{},
	}
	pkgs, err := g.loadPackages()
	req.NoError(err)

	err = g.parsePackages(pkgs)
	req.NoError(err)

	err = g.analyze()
	req.NoError(err)

	// this is g.render without writing to disk
	var buf bytes.Buffer
	err = g.execTemplate(&buf, equalsTmpl)
	req.NoError(err)

	formatted := g.format(buf.Bytes())
	got := string(formatted)

	req.Contains(got, "if s.Name != other.Name {")
	req.Contains(got, "if s.TaskName != other.TaskName {")

	req.Contains(got, "if !s.Connect.Equals(other.Connect) {")

	// TODO: need a struct member
	// require.Contains(got, "if !s.Foo.Equals(other.Foo) {")

	req.Contains(got, `
	for i, v := range s.Tags {
		if v != other.Tags[i] {
			return false
		}
	}`)

	req.Contains(got, `
	for i, v := range s.Checks {
		if !v.Equals(other.Checks[i]) {
			return false
		}
	}`)

	req.Contains(got, `
	for k, v := range s.Meta {
		v2, ok := other.Meta[k]
		if !ok {
			return false
		}
		if v != v2 {
			return false
		}
	}`)
}

func TestGenerate_Copy(t *testing.T) {
	req := require.New(t)

	g := &Generator{
		typeNames:      []string{"Job"},
		packageDir:     "../../nomad/structs",
		methods:        []string{"Job.All"},
		excludedFields: []string{"Job.Payload"},
		typeSpecs:      map[string]*TypeSpecNode{},
	}

	pkgs, err := g.loadPackages()
	req.NoError(err)

	err = g.parsePackages(pkgs)
	req.NoError(err)

	err = g.analyze()
	req.NoError(err)

	// this is g.render without writing to disk
	var buf bytes.Buffer
	err = g.execTemplate(&buf, copyTmpl)
	req.NoError(err)

	formatted := g.format(buf.Bytes())
	got := string(formatted)

	req.Contains(got, "xx.Multiregion = j.Multiregion.Copy()")
	req.Contains(got, "xx.Meta = helper.CopyMapStringString(j.Meta)")
	req.Contains(got, `
	xx.Affinities = make([]*Affinity, len(j.Affinities))
	for _, v := range j.Affinities {
		xx.Affinities = append(xx.Affinities, v.Copy())
	}
`)

}

func TestAnalyze_Copy(t *testing.T) {
	req := require.New(t)

	g := &Generator{
		packageDir:     "../../nomad/structs",
		typeNames:      []string{"Job"},
		methods:        []string{"Job.Copy"},
		excludedFields: []string{"Payload"},
		typeSpecs:      map[string]*TypeSpecNode{},
	}

	pkgs, err := g.loadPackages()
	req.NoError(err)

	err = g.parsePackages(pkgs)
	req.NoError(err)

	err = g.analyze()
	req.NoError(err)

	req.True(g.typeSpecs["Job"].isCopier())
	req.Len(g.typeSpecs["Job"].fields, 3) // 3 pointer fields

	got := func(typespec string) bool {
		return g.typeSpecs[typespec].isCopier()
	}

	req.True(got("Multiregion"), "Multiregion has pointer and array fields")
	req.True(got("PeriodicConfig"), "PeriodicConfig has a pointer field")
	req.True(got("ParameterizedJobConfig"), "ParameterizedJobConfig has array fields")
	req.True(got("UpdateStrategy"), "UpdateStrategy has a Copy method")
	req.False(got("TaskGroupSummary"), "TaskGroupSummary has only primitive fields")
}

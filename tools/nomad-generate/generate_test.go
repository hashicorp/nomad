package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_Equals(t *testing.T) {
	require := require.New(t)

	g := &Generator{
		typeNames:  []string{"Service"},
		packageDir: "../../nomad/structs",
		methods:    []string{"Service.Equals"},
		// excludedFields: []string{"Job.Payload", "Job.Stop", "Job.CreateIndex"},
		typeSpecs: map[string]*TypeSpecNode{},
	}
	pkgs, err := g.loadPackages()
	require.NoError(err)

	err = g.parsePackages(pkgs)
	require.NoError(err)

	err = g.analyze()
	require.NoError(err)

	// this is g.render without writing to disk
	var buf bytes.Buffer
	err = g.write(&buf, equalsTmpl)
	require.NoError(err)

	formatted := g.format(buf.Bytes())
	got := string(formatted)

	require.Contains(got, "if s.Name != other.Name {")
	require.Contains(got, "if s.TaskName != other.TaskName {")

	require.Contains(got, "if !s.Connect.Equals(other.Connect) {")

	// TODO: need a struct member
	// require.Contains(got, "if !s.Foo.Equals(other.Foo) {")

	require.Contains(got, `
	for i, v := range s.Tags {
		if v != other.Tags[i] {
			return false
		}
	}`)

	require.Contains(got, `
	for i, v := range s.Checks {
		if !v.Equals(other.Checks[i]) {
			return false
		}
	}`)

	require.Contains(got, `
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
	require := require.New(t)

	g := &Generator{
		typeNames:      []string{"Job"},
		packageDir:     "../../nomad/structs",
		methods:        []string{"Job.All"},
		excludedFields: []string{"Job.Payload"},
		typeSpecs:      map[string]*TypeSpecNode{},
	}

	pkgs, err := g.loadPackages()
	require.NoError(err)

	err = g.parsePackages(pkgs)
	require.NoError(err)

	err = g.analyze()
	require.NoError(err)

	// this is g.render without writing to disk
	var buf bytes.Buffer
	err = g.write(&buf, copyTmpl)
	require.NoError(err)

	formatted := g.format(buf.Bytes())
	got := string(formatted)

	require.Contains(got, "xx.Multiregion = j.Multiregion.Copy()")
	require.Contains(got, "xx.Meta = helper.CopyMapStringString(j.Meta)")
	require.Contains(got, `
	xx.Affinities = make([]*Affinity, len(j.Affinities))
	for _, v := range j.Affinities {
		xx.Affinities = append(xx.Affinities, v.Copy())
	}
`)

}

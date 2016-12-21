package template

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"

	dep "github.com/hashicorp/consul-template/dependency"
)

type Template struct {
	// Path is the path to this template on disk.
	Path string

	// LeftDelim and RightDelim are the left and right delimiters to use.
	LeftDelim, RightDelim string

	// Contents is the string contents for the template. It is either given
	// during template creation or read from disk when initialized.
	Contents string

	// HexMD5 stores the hex version of the MD5
	HexMD5 string
}

// NewTemplate creates and parses a new Consul Template template at the given
// path. If the template does not exist, an error is returned. During
// initialization, the template is read and is parsed for dependencies. Any
// errors that occur are returned.
func NewTemplate(path, contents, leftDelim, rightDelim string) (*Template, error) {
	// Validate that we are either given the path or the explicit contents
	pathEmpty, contentsEmpty := path == "", contents == ""
	if !pathEmpty && !contentsEmpty {
		return nil, errors.New("Either specify template path or content, not both")
	} else if pathEmpty && contentsEmpty {
		return nil, errors.New("Must specify template path or content")
	}

	template := &Template{
		Path:       path,
		Contents:   contents,
		LeftDelim:  leftDelim,
		RightDelim: rightDelim,
	}
	if err := template.init(); err != nil {
		return nil, err
	}

	return template, nil
}

// ID returns an identifier for the template
func (t *Template) ID() string {
	return t.HexMD5
}

// Execute evaluates this template in the context of the given brain.
//
// The first return value is the list of used dependencies.
// The second return value is the list of missing dependencies.
// The third return value is the rendered text.
// The fourth return value any error that occurs.
func (t *Template) Execute(brain *Brain) ([]dep.Dependency, []dep.Dependency, []byte, error) {
	usedMap := make(map[string]dep.Dependency)
	missingMap := make(map[string]dep.Dependency)
	name := filepath.Base(t.Path)
	funcs := funcMap(brain, usedMap, missingMap)

	tmpl, err := template.New(name).
		Delims(t.LeftDelim, t.RightDelim).
		Funcs(funcs).
		Parse(t.Contents)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("template: %s", err)
	}

	// TODO: accept an io.Writer instead
	buff := new(bytes.Buffer)
	if err := tmpl.Execute(buff, nil); err != nil {
		return nil, nil, nil, fmt.Errorf("template: %s", err)
	}

	// Update this list of this template's dependencies
	var used []dep.Dependency
	for _, dep := range usedMap {
		used = append(used, dep)
	}

	// Compile the list of missing dependencies
	var missing []dep.Dependency
	for _, dep := range missingMap {
		missing = append(missing, dep)
	}

	return used, missing, buff.Bytes(), nil
}

// init reads the template file and initializes required variables.
func (t *Template) init() error {
	// Render the template
	if t.Path != "" {
		contents, err := ioutil.ReadFile(t.Path)
		if err != nil {
			return err
		}
		t.Contents = string(contents)
	}

	// Compute the MD5, encode as hex
	hash := md5.Sum([]byte(t.Contents))
	t.HexMD5 = hex.EncodeToString(hash[:])

	return nil
}

// funcMap is the map of template functions to their respective functions.
func funcMap(brain *Brain, used, missing map[string]dep.Dependency) template.FuncMap {
	return template.FuncMap{
		// API functions
		"datacenters":    datacentersFunc(brain, used, missing),
		"file":           fileFunc(brain, used, missing),
		"key":            keyFunc(brain, used, missing),
		"key_exists":     keyExistsFunc(brain, used, missing),
		"key_or_default": keyWithDefaultFunc(brain, used, missing),
		"ls":             lsFunc(brain, used, missing),
		"node":           nodeFunc(brain, used, missing),
		"nodes":          nodesFunc(brain, used, missing),
		"secret":         secretFunc(brain, used, missing),
		"secrets":        secretsFunc(brain, used, missing),
		"service":        serviceFunc(brain, used, missing),
		"services":       servicesFunc(brain, used, missing),
		"tree":           treeFunc(brain, used, missing),
		"vault":          vaultFunc(brain, used, missing),

		// Helper functions
		"byKey":           byKey,
		"byTag":           byTag,
		"contains":        contains,
		"env":             env,
		"explode":         explode,
		"in":              in,
		"loop":            loop,
		"join":            join,
		"trimSpace":       trimSpace,
		"parseBool":       parseBool,
		"parseFloat":      parseFloat,
		"parseInt":        parseInt,
		"parseJSON":       parseJSON,
		"parseUint":       parseUint,
		"plugin":          plugin,
		"regexReplaceAll": regexReplaceAll,
		"regexMatch":      regexMatch,
		"replaceAll":      replaceAll,
		"timestamp":       timestamp,
		"toLower":         toLower,
		"toJSON":          toJSON,
		"toJSONPretty":    toJSONPretty,
		"toTitle":         toTitle,
		"toTOML":          toTOML,
		"toUpper":         toUpper,
		"toYAML":          toYAML,
		"split":           split,

		// Math functions
		"add":      add,
		"subtract": subtract,
		"multiply": multiply,
		"divide":   divide,
	}
}

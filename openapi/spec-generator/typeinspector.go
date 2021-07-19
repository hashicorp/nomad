package main

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"sync"
)

func NewTypeInspector(cfg *PackageConfig, logger loggerFunc) (*TypeInspector, error) {
	i := &TypeInspector{
		schemaRefAdapter: &schemaRefAdapter{
			SchemaRefs: make(map[string]*openapi3.SchemaRef),
			types:      make(map[string]*types.Type),
			typesMutex: sync.RWMutex{},
			analyzer:   nil,
		},
	}

	if err := i.init(cfg, logger); err != nil {
		return nil, err
	}

	return i, nil
}

type TypeInspector struct {
	fileSet          *token.FileSet
	pkg              *packages.Package
	logger           loggerFunc
	schemaRefAdapter *schemaRefAdapter
	structs          map[string]*types.Object
}

func (t *TypeInspector) init(cfg *PackageConfig, logger loggerFunc) error {
	t.logger = logger
	t.fileSet = token.NewFileSet()
	var err error
	var pkgs []*packages.Package
	if pkgs, err = packages.Load(&cfg.Config, cfg.Pattern); err != nil {
		return fmt.Errorf("TypeInspector.init.packages.load: %#v", err)
	}

	for _, pkg := range pkgs {
		t.pkg = pkg
		for _, goFile := range pkg.GoFiles {
			// positions are relative to fset
			_, err = parser.ParseFile(t.fileSet, goFile, nil, 0)
			if err != nil {
				return fmt.Errorf("TypeInspector.init.parser.parseGoFile: %#v\n", err)
			}
		}
	}

	return nil
}

func (t *TypeInspector) Inspect(typeName string) error {
	var typ types.Type
	var entry *types.Struct
	for id, def := range t.pkg.TypesInfo.Defs {
		if id.Name == typeName {
			var ok bool
			t.logger(id.Pos(), "defines", def)
			t.lookupParent(id)
			if entry, ok = t.toStruct(def); ok {
				if _, ok := t.structs[def.String()]; ok {
					t.structs[def.String()] = &def
				}
				typ = def.Type()
				t.logger("entry", typeName, "is def\n", entry)
			}
		}
	}

	if entry == nil {
		for id, used := range t.pkg.TypesInfo.Uses {
			if id.Name == typeName {
				var ok bool
				t.logger(id, "uses", used)
				if entry, ok = t.toStruct(used); ok {
					if _, ok := t.structs[used.String()]; ok {
						t.structs[used.String()] = &used
					}
					typ = used.Type()
					t.logger("entry", typeName, "is used\n", entry)
				}
			}
		}
	}

	if entry == nil {
		return fmt.Errorf("%s not found", typeName)
	}

	t.inspectFields(entry)

	ref, err := t.schemaRefAdapter.generateWithoutSaving(nil, typ)
	if err != nil {
		return err
	}
	t.logger("schemaRef", ref)

	return nil
}

func (t *TypeInspector) inspectFields(structType *types.Struct) {
	for i := 0; i < structType.NumFields(); i++ {
		t.logger("---FIELD---")
		t.logger("name:", structType.Field(i).Name())
		t.logger("type:", structType.Field(i).Type())
		t.logger("pkg:", structType.Field(i).Pkg())

		if fieldNamed, ok := structType.Field(i).Type().(*types.Named); ok {
			if _, ok := t.toStruct(fieldNamed.Obj()); ok {
				if err := t.Inspect(fieldNamed.Obj().Name()); err != nil {
					t.logger(err)
				}
			}
		}
	}
}

func (t TypeInspector) toStruct(obj types.Object) (*types.Struct, bool) {
	s, ok := obj.Type().Underlying().(*types.Struct)
	return s, ok
}

func (t *TypeInspector) lookupParent(id *ast.Ident) {
	for _, scope := range t.pkg.TypesInfo.Scopes {
		if scope.Contains(id.Pos()) {
			if _, obj := scope.LookupParent(id.Name, id.Pos()); obj != nil {
				t.logger(id, "found parent", obj.Type())
			} else {
				t.logger(id, "parent not found")
			}
		}
	}
}

func (t *TypeInspector) lookupSelections(expr *ast.SelectorExpr) {
	for sx, selection := range t.pkg.TypesInfo.Selections {
		if sx == expr {
			t.logger(expr, "found selection", selection)
		}
	}
}

func (t *TypeInspector) printSchemaRefs() {
	for _, r := range t.schemaRefAdapter.SchemaRefs {
		t.logger(r)
	}
}

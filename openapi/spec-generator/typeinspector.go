package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
)

func NewTypeInspector(cfg *PackageConfig, logger loggerFunc) (*TypeInspector, error) {
	i := &TypeInspector{}

	if err := i.init(cfg, logger); err != nil {
		return nil, err
	}

	return i, nil
}

type TypeInspector struct {
	fileSet *token.FileSet
	pkg     *packages.Package
	logger  loggerFunc
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
	var entry *types.Struct
	for id, def := range t.pkg.TypesInfo.Defs {
		if id.Name == typeName {
			var ok bool
			if entry, ok = t.toStruct(def); ok {
				t.logger("entry", typeName, "is def\n", entry)
				break
			}
		}
	}

	if entry == nil {
		for id, used := range t.pkg.TypesInfo.Uses {
			if id.Name == typeName {
				var ok bool
				if entry, ok = t.toStruct(used); ok {
					t.logger("entry", typeName, "is used\n", entry)
					break
				}
			}
		}
	}

	if entry == nil {
		return fmt.Errorf("%s not found", typeName)
	}

	t.inspectFields(entry)

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

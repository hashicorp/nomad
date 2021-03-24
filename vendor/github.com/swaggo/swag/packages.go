package swag

import (
	"go/ast"
	"go/token"
	"strings"
)

//PackagesDefinitions map[package import path]*PackageDefinitions
type PackagesDefinitions struct {
	files             map[*ast.File]*AstFileInfo
	packages          map[string]*PackageDefinitions
	uniqueDefinitions map[string]*TypeSpecDef
}

//NewPackagesDefinitions create object PackagesDefinitions
func NewPackagesDefinitions() *PackagesDefinitions {
	return &PackagesDefinitions{
		files:             make(map[*ast.File]*AstFileInfo),
		packages:          make(map[string]*PackageDefinitions),
		uniqueDefinitions: make(map[string]*TypeSpecDef),
	}
}

//CollectAstFile collect ast.file
func (pkgs *PackagesDefinitions) CollectAstFile(packageDir, path string, astFile *ast.File) {
	if pkgs.files == nil {
		pkgs.files = make(map[*ast.File]*AstFileInfo)
	}

	pkgs.files[astFile] = &AstFileInfo{
		File:        astFile,
		Path:        path,
		PackagePath: packageDir,
	}

	if len(packageDir) == 0 {
		return
	}

	if pkgs.packages == nil {
		pkgs.packages = make(map[string]*PackageDefinitions)
	}

	if pd, ok := pkgs.packages[packageDir]; ok {
		pd.Files[path] = astFile
	} else {
		pkgs.packages[packageDir] = &PackageDefinitions{
			Name:            astFile.Name.Name,
			Files:           map[string]*ast.File{path: astFile},
			TypeDefinitions: make(map[string]*TypeSpecDef),
		}
	}
}

//RangeFiles for range the collection of ast.File
func (pkgs *PackagesDefinitions) RangeFiles(handle func(filename string, file *ast.File) error) error {
	for file, info := range pkgs.files {
		if err := handle(info.Path, file); err != nil {
			return err
		}
	}
	return nil
}

//ParseTypes parse types
//@Return parsed definitions
func (pkgs *PackagesDefinitions) ParseTypes() (map[*TypeSpecDef]*Schema, error) {
	parsedSchemas := make(map[*TypeSpecDef]*Schema)
	for astFile, info := range pkgs.files {
		for _, astDeclaration := range astFile.Decls {
			if generalDeclaration, ok := astDeclaration.(*ast.GenDecl); ok && generalDeclaration.Tok == token.TYPE {
				for _, astSpec := range generalDeclaration.Specs {
					if typeSpec, ok := astSpec.(*ast.TypeSpec); ok {
						typeSpecDef := &TypeSpecDef{
							PkgPath:  info.PackagePath,
							File:     astFile,
							TypeSpec: typeSpec,
						}

						if idt, ok := typeSpec.Type.(*ast.Ident); ok && IsGolangPrimitiveType(idt.Name) {
							parsedSchemas[typeSpecDef] = &Schema{
								PkgPath: typeSpecDef.PkgPath,
								Name:    astFile.Name.Name,
								Schema:  PrimitiveSchema(TransToValidSchemeType(idt.Name)),
							}
						}

						if pkgs.uniqueDefinitions == nil {
							pkgs.uniqueDefinitions = make(map[string]*TypeSpecDef)
						}

						fullName := typeSpecDef.FullName()
						anotherTypeDef, ok := pkgs.uniqueDefinitions[fullName]
						if ok {
							if typeSpecDef.PkgPath == anotherTypeDef.PkgPath {
								continue
							} else {
								delete(pkgs.uniqueDefinitions, fullName)
							}
						} else {
							pkgs.uniqueDefinitions[fullName] = typeSpecDef
						}

						pkgs.packages[typeSpecDef.PkgPath].TypeDefinitions[typeSpecDef.Name()] = typeSpecDef
					}
				}
			}
		}
	}
	return parsedSchemas, nil
}

func (pkgs *PackagesDefinitions) findTypeSpec(pkgPath string, typeName string) *TypeSpecDef {
	if pkgs.packages != nil {
		if pd, ok := pkgs.packages[pkgPath]; ok {
			if typeSpec, ok := pd.TypeDefinitions[typeName]; ok {
				return typeSpec
			}
		}
	}
	return nil
}

// findPackagePathFromImports finds out the package path of a package via ranging imports of a ast.File
// @pkg the name of the target package
// @file current ast.File in which to search imports
// @return the package path of a package of @pkg
func (pkgs *PackagesDefinitions) findPackagePathFromImports(pkg string, file *ast.File) string {
	if file == nil {
		return ""
	}

	if strings.ContainsRune(pkg, '.') {
		pkg = strings.Split(pkg, ".")[0]
	}

	hasAnonymousPkg := false

	// prior to match named package
	for _, imp := range file.Imports {
		if imp.Name != nil {
			if imp.Name.Name == pkg {
				return strings.Trim(imp.Path.Value, `"`)
			} else if imp.Name.Name == "_" {
				hasAnonymousPkg = true
			}
		} else if pkgs.packages != nil {
			path := strings.Trim(imp.Path.Value, `"`)
			if pd, ok := pkgs.packages[path]; ok {
				if pd.Name == pkg {
					return path
				}
			}
		}
	}

	//match unnamed package
	if hasAnonymousPkg && pkgs.packages != nil {
		for _, imp := range file.Imports {
			if imp.Name == nil {
				continue
			}
			if imp.Name.Name == "_" {
				path := strings.Trim(imp.Path.Value, `"`)
				if pd, ok := pkgs.packages[path]; ok {
					if pd.Name == pkg {
						return path
					}
				}
			}
		}
	}
	return ""
}

// FindTypeSpec finds out TypeSpecDef of a type by typeName
// @typeName the name of the target type, if it starts with a package name, find its own package path from imports on top of @file
// @file the ast.file in which @typeName is used
// @pkgPath the package path of @file
func (pkgs *PackagesDefinitions) FindTypeSpec(typeName string, file *ast.File) *TypeSpecDef {
	if IsGolangPrimitiveType(typeName) {
		return nil
	} else if file == nil { // for test
		return pkgs.uniqueDefinitions[typeName]
	}

	if strings.ContainsRune(typeName, '.') {
		parts := strings.Split(typeName, ".")

		isAliasPkgName := func(file *ast.File, pkgName string) bool {
			if file != nil && file.Imports != nil {
				for _, pkg := range file.Imports {
					if pkg.Name != nil && pkg.Name.Name == pkgName {
						return true
					}
				}
			}

			return false
		}

		if !isAliasPkgName(file, parts[0]) {
			if typeDef, ok := pkgs.uniqueDefinitions[typeName]; ok {
				return typeDef
			}
		}

		pkgPath := pkgs.findPackagePathFromImports(parts[0], file)
		if len(pkgPath) == 0 && parts[0] == file.Name.Name {
			pkgPath = pkgs.files[file].PackagePath
		}
		return pkgs.findTypeSpec(pkgPath, parts[1])
	}

	if typeDef, ok := pkgs.uniqueDefinitions[fullTypeName(file.Name.Name, typeName)]; ok {
		return typeDef
	}

	if typeDef := pkgs.findTypeSpec(pkgs.files[file].PackagePath, typeName); typeDef != nil {
		return typeDef
	}

	for _, imp := range file.Imports {
		if imp.Name != nil && imp.Name.Name == "." {
			pkgPath := strings.Trim(imp.Path.Value, `"`)
			if typeDef := pkgs.findTypeSpec(pkgPath, typeName); typeDef != nil {
				return typeDef
			}
		}
	}

	return nil
}

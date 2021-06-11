package openapigen

import (
	"fmt"
	"go/importer"
)

func GenerateAPIModels() {
	pkg, err := importer.Default().Import("github.com/hashicorp/nomad/api")
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		return
	}
	for _, declName := range pkg.Scope().Names() {
		fmt.Println(declName)
	}
}
package vault

import (
	"errors"
	"fmt"
	"os"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// GenVaultFunc returns generated vault func
var GenVaultFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "path",
			Type: cty.String,
		},
		{
			Name: "key",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		path := args[0].AsString()
		key := args[1].AsString()

		return vault(path, key)
	},
})

func vault(path string, key string) (cty.Value, error) {

	if token := os.Getenv("VAULT_TOKEN"); token == "" {
		return cty.StringVal(""), errors.New("Must set VAULT_TOKEN env var in order to use vault template function")
	}

	vaultConfig := vaultapi.DefaultConfig()
	cli, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return cty.StringVal(""), fmt.Errorf("Error getting Vault client: %s", err)
	}
	secret, err := cli.Logical().Read(path)
	if err != nil {
		return cty.StringVal(""), fmt.Errorf("Error reading vault secret: %s", err)
	}
	if secret == nil {
		return cty.StringVal(""), errors.New("Vault Secret does not exist at the given path")
	}

	data, ok := secret.Data["data"]
	if !ok {
		// maybe this is v1, not v2 kv store
		value, ok := secret.Data[key]
		if ok {
			return cty.StringVal(value.(string)), nil
		}

		// neither v1 nor v2 produced a valid value
		return cty.StringVal(""), fmt.Errorf("Vault data was empty at the given path. Warnings: %s", strings.Join(secret.Warnings, "; "))
	}

	if val, ok := data.(map[string]interface{})[key]; ok {
		return cty.StringVal(val.(string)), nil
	}
	return cty.StringVal(""), errors.New("Vault path does not contain the requested key")
}

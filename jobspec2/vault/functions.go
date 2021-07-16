package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

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

func strOrJSON(data map[string]interface{}, key string) (value cty.Value, err error) {
	v, ok := data[key]
	if !ok {
		return cty.StringVal(""), fmt.Errorf("Vault path does not contain the requested key")
	}
	switch v.(type) {
	case string:
		return cty.StringVal(v.(string)), nil
	default:
		s, err := json.Marshal(v)
		if err != nil {
			return cty.StringVal(""), fmt.Errorf("error marshalling secret into json %s", err)
		}
		return cty.StringVal(string(s)), nil
	}
}

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
		return cty.StringVal(""), fmt.Errorf("Vault Secret does not exist at %s", path)
	}

	data, ok := secret.Data["data"]
	if !ok {
		// maybe this is v1, not v2 kv store
		return strOrJSON(secret.Data, key)
	}

	return strOrJSON(data.(map[string]interface{}), key)
}

package secrets

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

type AwsProvider struct {
	secret   *structs.Secret
	tmplPath string
	config   *nomadProviderConfig
}

// NewNomadProvider takes a task secret and decodes the config, overwriting the default config fields
// with any provided fields, returning an error if the secret or secret's config is invalid.
func NewAWSProvider(secret *structs.Secret, path string, namespace string) (*AwsProvider, error) {
	if secret == nil {
		return nil, fmt.Errorf("empty secret for nomad provider")
	}

	conf := defaultNomadConfig(namespace)
	if err := mapstructure.Decode(secret.Config, conf); err != nil {
		return nil, err
	}

	return &AwsProvider{
		config:   conf,
		secret:   secret,
		tmplPath: path,
	}, nil
}

func (n *AwsProvider) BuildTemplate() *structs.Template {

	// fmt.Println("fetching aws secret")
	// secretName := "MyTestSecret"
	// region := "us-east-2"
	//
	// client := &http.Client{
	// 	Transport: cleanhttp.DefaultTransport(),
	// }
	// cfg, err := config.LoadDefaultConfig(context.Background(),
	// 	config.WithRegion(region),
	// 	config.WithHTTPClient(client),
	// 	config.WithRetryMaxAttempts(0),
	// )
	// if err != nil {
	// 	return nil
	// }
	//
	// sm := secretsmanager.NewFromConfig(cfg)
	//
	// val, err := sm.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
	// 	SecretId:     aws.String(secretName),
	// 	VersionStage: aws.String("AWSCURRENT"),
	// })
	// if err != nil {
	// 	fmt.Println(err)
	// 	return nil
	// }
	//
	// var res string = *val.SecretString
	//
	// fmt.Println(res)

	return &structs.Template{}
}

func (n *AwsProvider) Parse() (map[string]string, error) {
	return nil, nil
	// dest := filepath.Clean(n.tmplPath)
	// f, err := os.Open(dest)
	// if err != nil {
	// 	return nil, fmt.Errorf("error opening env template: %v", err)
	// }
	// defer func() {
	// 	f.Close()
	// 	os.Remove(dest)
	// }()
	//
	// return envparse.Parse(f)
}

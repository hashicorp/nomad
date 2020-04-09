package resolvconf

import (
	"bytes"
	"io/ioutil"
	"strings"
)

type ResolvConf struct {
	servers  []string
	searches []string
	options  []string
}

func New(servers []string, searches []string, options []string) (*ResolvConf, error) {
	return &ResolvConf{
		servers:  servers,
		searches: searches,
		options:  options,
	}
	return nil, nil
}

func (rc *ResolvConf) Content() []byte {
	content := bytes.NewBuffer(nil)
	if len(rc.searches) > 0 {
		if searchString := strings.Join(rc.searches, " "); strings.Trim(searchString, " ") != "." {
			content.WriteString("search " + searchString + "\n")
		}
	}
	for _, dns := range rc.servers {
		content.WriteString("nameserver " + dns + "\n")
	}
	if len(rc.options) > 0 {
		if optsString := strings.Join(rc.options, " "); strings.Trim(optsString, " ") != "" {
			content.WriteString("options " + optsString + "\n")
		}
	}

	return content.Bytes()
}

func (rc *ResolvConf) WriteToPath(path string) error {
	return ioutil.WriteFile(path, rc.Content(), 0644)
}

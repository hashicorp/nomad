package jobspec

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/hashicorp/nomad/api"
)

func LoadContext(j *api.Job, dir string) error {
	for _, groups := range j.TaskGroups {
		for _, task := range groups.Tasks {
			for _, tmpl := range task.Templates {
				if tmpl.LocalSource == nil {
					continue
				}

				// Load LocalSource file into tmpl.Data
				filepath := *tmpl.LocalSource
				if !path.IsAbs(filepath) {
					filepath = path.Join(dir, filepath)
				}
				rd, err := os.OpenFile(filepath, os.O_RDONLY, 0)
				if err != nil {
					return err
				}

				// Buffer file into memory as a string
				data, err := ioutil.ReadAll(rd)
				if err != nil {
					return err
				}
				str := string(data)
				tmpl.EmbeddedTmpl = &str

				err = rd.Close()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

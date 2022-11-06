package subproc

import "os"

func Self() string {
	s, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return s
}

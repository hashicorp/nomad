package driver

import (
	"fmt"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
)

func getBindDirs(alloc *allocdir.AllocDir, task *structs.Task) ([]string, error) {
	shared := alloc.SharedDir
	local, ok := alloc.TaskDirs[task.Name]
	if !ok {
		return nil, fmt.Errorf("Failed to find task local directory: %v", task.Name)
	}

	return []string{
		// "z" and "Z" option is to allocate directory with SELinux label.
		fmt.Sprintf("%s:/%s:rw,z", shared, allocdir.SharedAllocName),
		// capital "Z" will label with Multi-Category Security (MCS) labels
		fmt.Sprintf("%s:/%s:rw,Z", local, allocdir.TaskLocal),
	}, nil
}

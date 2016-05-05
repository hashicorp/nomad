package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

const (
	AnnotationForcesCreate            = "forces create"
	AnnotationForcesDestroy           = "forces destroy"
	AnnotationForcesInplaceUpdate     = "forces in-place update"
	AnnotationForcesDestructiveUpdate = "forces create/destroy update"
)

// Annotate takes the diff between the old and new version of a Job and will add
// annotations to the diff to aide human understanding of the plan.
//
// Currently the things that are annotated are:
// * Count up and count down changes
// * Task changes will be annotated with:
//    * forces create/destroy update
//    * forces in-place update
func Annotate(diff *structs.JobDiff) {
	// No annotation needed as the job was either just submitted or deleted.
	if diff.Type != structs.DiffTypeEdited {
		return
	}

	tgDiffs := diff.TaskGroups
	if tgDiffs == nil {
		return
	}

	for _, tgDiff := range tgDiffs.Edited {
		annotateTaskGroup(tgDiff)
	}
}

// annotateTaskGroup takes a task group diff and annotates it.
func annotateTaskGroup(diff *structs.TaskGroupDiff) {
	// Don't annotate unless the task group was edited. If it was a
	// create/destroy it is not needed.
	if diff == nil || diff.Type != structs.DiffTypeEdited {
		return
	}

	// Annotate the count
	annotateCountChange(diff)

	// Annotate the tasks.
	taskDiffs := diff.Tasks
	if taskDiffs == nil {
		return
	}

	for _, taskDiff := range taskDiffs.Edited {
		annotateTask(taskDiff)
	}
}

// annotateCountChange takes a task group diff and annotates the count
// parameter.
func annotateCountChange(diff *structs.TaskGroupDiff) {
	// Don't annotate unless the task was edited. If it was a create/destroy it
	// is not needed.
	if diff == nil || diff.Type != structs.DiffTypeEdited {
		return
	}

	countDiff, ok := diff.PrimitiveFields["Count"]
	if !ok {
		return
	}
	oldV := countDiff.OldValue.(int)
	newV := countDiff.NewValue.(int)

	if oldV < newV {
		countDiff.Annotations = append(countDiff.Annotations, AnnotationForcesCreate)
	} else if newV < oldV {
		countDiff.Annotations = append(countDiff.Annotations, AnnotationForcesDestroy)
	}
}

// annotateCountChange takes a task diff and annotates it.
func annotateTask(diff *structs.TaskDiff) {
	// Don't annotate unless the task was edited. If it was a create/destroy it
	// is not needed.
	if diff == nil || diff.Type != structs.DiffTypeEdited {
		return
	}

	// All changes to primitive fields result in a destructive update.
	destructive := false
	if len(diff.PrimitiveFields) != 0 {
		destructive = true
	}

	if diff.Env != nil ||
		diff.Meta != nil ||
		diff.Artifacts != nil ||
		diff.Resources != nil ||
		diff.Config != nil {
		destructive = true
	}

	if destructive {
		diff.Annotations = append(diff.Annotations, AnnotationForcesDestructiveUpdate)
	} else {
		diff.Annotations = append(diff.Annotations, AnnotationForcesInplaceUpdate)
	}
}

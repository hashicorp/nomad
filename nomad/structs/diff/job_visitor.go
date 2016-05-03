package diff

// JobVisitor is the set of types a visitor must implement to traverse a JobDiff
// structure.
type JobVisitor interface {
	VisitJob(*JobDiff)
	VisitTaskGroups(*TaskGroupsDiff)
	VisitTaskGroup(*TaskGroupDiff)
	VisitTasks(*TasksDiff)
	VisitTask(*TaskDiff)
	VisitServices(*ServicesDiff)
	VisitService(*ServiceDiff)
	VisitServiceChecks(*ServiceChecksDiff)
	VisitServiceCheck(*ServiceCheckDiff)
	VisitTaskArtifactsDiff(*TaskArtifactsDiff)
	VisitTaskArtifactDiff(*TaskArtifactDiff)
	VisitResources(*ResourcesDiff)
	VisitNetworkResources(*NetworkResourcesDiff)
	VisitNetworkResource(*NetworkResourceDiff)
	VisitPorts(*PortsDiff)
	VisitPrimitiveStruct(*PrimitiveStructDiff)
	VisitField(*FieldDiff)
	VisitStringSet(*StringSetDiff)
	VisitStringMap(*StringMapDiff)
	VisitStringValueDelta(*StringValueDelta)
}

// JobDiffComponent is the method a diff component must implement to be visited.
type JobDiffComponent interface {
	Accept(JobVisitor)
}

func (j *JobDiff) Accept(v JobVisitor)              { v.VisitJob(j) }
func (t *TaskGroupsDiff) Accept(v JobVisitor)       { v.VisitTaskGroups(t) }
func (t *TaskGroupDiff) Accept(v JobVisitor)        { v.VisitTaskGroup(t) }
func (t *TasksDiff) Accept(v JobVisitor)            { v.VisitTasks(t) }
func (t *TaskDiff) Accept(v JobVisitor)             { v.VisitTask(t) }
func (s *ServicesDiff) Accept(v JobVisitor)         { v.VisitServices(s) }
func (s *ServiceDiff) Accept(v JobVisitor)          { v.VisitService(s) }
func (s *ServiceChecksDiff) Accept(v JobVisitor)    { v.VisitServiceChecks(s) }
func (s *ServiceCheckDiff) Accept(v JobVisitor)     { v.VisitServiceCheck(s) }
func (t *TaskArtifactsDiff) Accept(v JobVisitor)    { v.VisitTaskArtifactsDiff(t) }
func (t *TaskArtifactDiff) Accept(v JobVisitor)     { v.VisitTaskArtifactDiff(t) }
func (r *ResourcesDiff) Accept(v JobVisitor)        { v.VisitResources(r) }
func (n *NetworkResourcesDiff) Accept(v JobVisitor) { v.VisitNetworkResources(n) }
func (n *NetworkResourceDiff) Accept(v JobVisitor)  { v.VisitNetworkResource(n) }
func (p *PortsDiff) Accept(v JobVisitor)            { v.VisitPorts(p) }
func (p *PrimitiveStructDiff) Accept(v JobVisitor)  { v.VisitPrimitiveStruct(p) }
func (f *FieldDiff) Accept(v JobVisitor)            { v.VisitField(f) }
func (s *StringSetDiff) Accept(v JobVisitor)        { v.VisitStringSet(s) }
func (s *StringMapDiff) Accept(v JobVisitor)        { v.VisitStringMap(s) }
func (s *StringValueDelta) Accept(v JobVisitor)     { v.VisitStringValueDelta(s) }

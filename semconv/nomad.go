package semconv

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"go.opentelemetry.io/otel/attribute"
)

const ()

// A Nomad Region.
const (
	NomadRegionKey = attribute.Key("nomad.region")
)

func NomadRegion(val string) attribute.KeyValue {
	return NomadRegionKey.String(val)
}

// A Nomad Namespace.
const (
	NomadNamespaceKey = attribute.Key("nomad.namespace")
)

func NomadNamespace(val string) attribute.KeyValue {
	return NomadNamespaceKey.String(val)
}

func Namespace(ns *structs.Namespace) []attribute.KeyValue {
	return []attribute.KeyValue{
		NomadNamespace(ns.Name),
	}
}

// A Nomad Job.
const (
	NomadJobIDKey   = attribute.Key("nomad.job.id")
	NomadJobNameKey = attribute.Key("nomad.job.name")
)

func NomadJobID(val string) attribute.KeyValue {
	return NomadJobIDKey.String(val)
}

func NomadJobName(val string) attribute.KeyValue {
	return NomadJobNameKey.String(val)
}

func Job(job *structs.Job) []attribute.KeyValue {
	return []attribute.KeyValue{
		NomadRegion(job.Region),
		NomadNamespace(job.Namespace),
		NomadJobID(job.ID),
		NomadJobName(job.Name),
	}
}

// A Nomad Evaluation.
const (
	NomadEvalIDKey          = attribute.Key("nomad.eval.id")
	NomadEvalTypeKey        = attribute.Key("nomad.eval.type")
	NomadEvalTriggeredByKey = attribute.Key("nomad.eval.triggered_by")
)

func NomadEvalID(val string) attribute.KeyValue {
	return NomadEvalIDKey.String(val)
}

func NomadEvalType(val string) attribute.KeyValue {
	return NomadEvalTypeKey.String(val)
}

func NomadEvalTriggeredBy(val string) attribute.KeyValue {
	return NomadEvalTriggeredByKey.String(val)
}

func Eval(eval *structs.Evaluation) []attribute.KeyValue {
	attr := []attribute.KeyValue{
		NomadNamespace(eval.Namespace),
		NomadJobID(eval.JobID),
		NomadEvalID(eval.ID),
		NomadEvalType(eval.Type),
		NomadEvalTriggeredBy(eval.TriggeredBy),
	}
	if len(eval.DeploymentID) != 0 {
		attr = append(attr, NomadDeploymentID(eval.DeploymentID))
	}
	if len(eval.NodeID) != 0 {
		attr = append(attr, NomadNodeID(eval.NodeID))
	}
	return attr
}

// A Nomad Deployment.
const (
	NomadDeploymentIDKey = attribute.Key("nomad.deployment.id")
)

func NomadDeploymentID(val string) attribute.KeyValue {
	return NomadDeploymentIDKey.String(val)
}

func Deployment(d *structs.Deployment) []attribute.KeyValue {
	return []attribute.KeyValue{
		NomadNamespace(d.Namespace),
		NomadDeploymentID(d.ID),
		NomadJobID(d.JobID),
	}
}

// A Nomad Allocation.
const (
	NomadAllocIDKey = attribute.Key("nomad.alloc.id")
)

func NomadAllocID(val string) attribute.KeyValue {
	return NomadAllocIDKey.String(val)
}

func Alloc(a *structs.Allocation) []attribute.KeyValue {
	attr := []attribute.KeyValue{
		NomadNamespace(a.Namespace),
		NomadAllocID(a.ID),
		NomadJobID(a.JobID),
		NomadEvalID(a.EvalID),
		NomadNodeID(a.NodeID),
	}
	if a.DeploymentID != "" {
		attr = append(attr, NomadDeploymentID(a.DeploymentID))
	}
	return attr
}

// A Nomad Task.
const (
	NomadTaskNameKey = attribute.Key("nomad.task.name")
)

func NomadTaskName(val string) attribute.KeyValue {
	return NomadTaskNameKey.String(val)
}

func Task(t *structs.Task) []attribute.KeyValue {
	return []attribute.KeyValue{NomadTaskName(t.Name)}
}

// A Nomad Node.
const (
	NomadNodeIDKey         = attribute.Key("nomad.node.id")
	NomadNodeNameKey       = attribute.Key("nomad.node.name")
	NomadNodeDatacenterKey = attribute.Key("nomad.node.datacenter")
	NomadNodeClassKey      = attribute.Key("nomad.node.class")
)

func NomadNodeID(val string) attribute.KeyValue {
	return NomadNodeIDKey.String(val)
}

func NomadNodeName(val string) attribute.KeyValue {
	return NomadNodeNameKey.String(val)
}

func NomadNodeDatacenter(val string) attribute.KeyValue {
	return NomadNodeDatacenterKey.String(val)
}

func NomadNodeClass(val string) attribute.KeyValue {
	return NomadNodeClassKey.String(val)
}

func Node(n *structs.Node) []attribute.KeyValue {
	attr := []attribute.KeyValue{
		NomadNodeID(n.ID),
		NomadNodeName(n.Name),
		NomadNodeDatacenter(n.Datacenter),
	}
	if n.NodeClass != "" {
		attr = append(attr, NomadNodeClass(n.NodeClass))
	}
	return attr
}

// A Nomad Scheduler.
const (
	NomadSchedulerKey = attribute.Key("nomad.scheduler")
)

var (
	NomadSchedulerBatch    = NomadSchedulerKey.String("batch")
	NomadSchedulerService  = NomadSchedulerKey.String("service")
	NomadSchedulerSysbatch = NomadSchedulerKey.String("sysbatch")
	NomadSchedulerSystem   = NomadSchedulerKey.String("system")
)

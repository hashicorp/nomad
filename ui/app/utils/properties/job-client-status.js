import { computed } from '@ember/object';

// An Ember.Computed property that persists set values in localStorage
// and will attempt to get its initial value from localStorage before
// falling back to a default.
//
// ex. showTutorial: localStorageProperty('nomadTutorial', true),

const STATUS = [
  'queued',
  'notScheduled',
  'starting',
  'running',
  'complete',
  'partial',
  'degraded',
  'failed',
  'lost',
];

export default function jobClientStatus(nodesKey, jobKey) {
  return computed(nodesKey, jobKey, function() {
    const job = this.get(jobKey);
    const nodes = this.get(nodesKey).filter(n => {
      return job.datacenters.indexOf(n.datacenter) >= 0;
    });

    if (job.status === 'pending') {
      return allQueued(nodes);
    }

    const allocsByNodeID = {};
    job.allocations.forEach(a => {
      const nodeId = a.node.get('id');
      if (!(nodeId in allocsByNodeID)) {
        allocsByNodeID[nodeId] = [];
      }
      allocsByNodeID[nodeId].push(a);
    });

    const result = {
      byNode: {},
      byStatus: {},
    };
    nodes.forEach(n => {
      const status = jobStatus(allocsByNodeID[n.id], job.taskGroups.length);
      result.byNode[n.id] = status;

      if (!(status in result.byStatus)) {
        result.byStatus[status] = [];
      }
      result.byStatus[status].push(n.id);
    });
    result.byStatus = canonicalizeStatus(result.byStatus);
    return result;
  });
}

function allQueued(nodes) {
  const nodeIDs = nodes.map(n => n.id);
  return {
    byNode: Object.fromEntries(nodeIDs.map(id => [id, 'queued'])),
    byStatus: canonicalizeStatus({ queued: nodeIDs }),
  };
}

function canonicalizeStatus(status) {
  for (let i = 0; i < STATUS.length; i++) {
    const s = STATUS[i];
    if (!(s in status)) {
      status[s] = [];
    }
  }
  return status;
}

function jobStatus(allocs, expected) {
  if (!allocs) {
    return 'notScheduled';
  }

  if (allocs.length < expected) {
    return 'partial';
  }

  const summary = allocs.reduce((acc, a) => {
    const status = a.clientStatus;
    if (!(status in acc)) {
      acc[status] = 0;
    }
    acc[status]++;
    return acc;
  }, {});

  const terminalStatus = ['failed', 'lost', 'complete'];
  for (let i = 0; i < terminalStatus.length; i++) {
    const s = terminalStatus[i];
    if (summary[s] === expected) {
      return s;
    }
  }

  if (summary['failed'] > 0 || summary['lost'] > 0) {
    return 'degraded';
  }

  if (summary['running'] > 0) {
    return 'running';
  }

  return 'starting';
}

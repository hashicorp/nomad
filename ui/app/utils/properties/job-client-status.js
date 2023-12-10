/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import matchGlob from '../match-glob';

const STATUS = [
  'queued',
  'notScheduled',
  'starting',
  'running',
  'complete',
  'degraded',
  'failed',
  'lost',
  'unknown',
];

// An Ember.Computed property that computes the aggregated status of a job in a
// client based on the desiredStatus of each allocation placed in the client.
//
// ex. clientStaus: jobClientStatus('nodes', 'job'),
export default function jobClientStatus(nodesKey, jobKey) {
  return computed(
    `${nodesKey}.[]`,
    `${jobKey}.{datacenters,status,allocations.@each.clientStatus,taskGroups}`,
    function () {
      const job = this.get(jobKey);
      const nodes = this.get(nodesKey);

      // Filter nodes by the datacenters defined in the job.
      const filteredNodes = nodes.filter((n) => {
        return job.datacenters.find((dc) => {
          return !!matchGlob(dc, n.datacenter);
        });
      });

      if (job.status === 'pending') {
        return allQueued(filteredNodes);
      }

      // Group the job allocations by the ID of the client that is running them.
      const allocsByNodeID = {};
      job.allocations.forEach((a) => {
        const nodeId = a.belongsTo('node').id();
        if (!allocsByNodeID[nodeId]) {
          allocsByNodeID[nodeId] = [];
        }
        allocsByNodeID[nodeId].push(a);
      });

      const result = {
        byNode: {},
        byStatus: {},
        totalNodes: filteredNodes.length,
      };
      filteredNodes.forEach((n) => {
        const status = jobStatus(allocsByNodeID[n.id], job.taskGroups.length);
        result.byNode[n.id] = status;

        if (!result.byStatus[status]) {
          result.byStatus[status] = [];
        }
        result.byStatus[status].push(n.id);
      });
      result.byStatus = canonicalizeStatus(result.byStatus);
      return result;
    }
  );
}

function allQueued(nodes) {
  const nodeIDs = nodes.map((n) => n.id);
  return {
    byNode: Object.fromEntries(nodeIDs.map((id) => [id, 'queued'])),
    byStatus: canonicalizeStatus({ queued: nodeIDs }),
    totalNodes: nodes.length,
  };
}

// canonicalizeStatus makes sure all possible statuses are present in the final
// returned object. Statuses missing from the input will be assigned an emtpy
// array.
function canonicalizeStatus(status) {
  for (let i = 0; i < STATUS.length; i++) {
    const s = STATUS[i];
    if (!status[s]) {
      status[s] = [];
    }
  }
  return status;
}

// jobStatus computes the aggregated status of a job in a client.
//
// `allocs` are the list of allocations for a job that are placed in a specific
// client.
// `expected` is the number of allocations the client should have.
function jobStatus(allocs, expected) {
  // The `pending` status has already been checked, so if at this point the
  // client doesn't have any allocations we assume that it was not considered
  // for scheduling for some reason.
  if (!allocs) {
    return 'notScheduled';
  }

  // If there are some allocations, but not how many we expected, the job is
  // considered `degraded` since it did fully run in this client.
  if (allocs.length < expected) {
    return 'degraded';
  }

  // Count how many allocations are in each `clientStatus` value.
  const summary = allocs
    .filter((a) => !a.isOld)
    .reduce((acc, a) => {
      const status = a.clientStatus;
      if (!acc[status]) {
        acc[status] = 0;
      }
      acc[status]++;
      return acc;
    }, {});

  // Theses statuses are considered terminal, i.e., an allocation will never
  // move from this status to another.
  // If all of the expected allocations are in one of these statuses, the job
  // as a whole is considered to be in the same status.
  const terminalStatuses = ['failed', 'lost', 'complete'];
  for (let i = 0; i < terminalStatuses.length; i++) {
    const s = terminalStatuses[i];
    if (summary[s] === expected) {
      return s;
    }
  }

  // It only takes one allocation to be in one of these statuses for the
  // entire job to be considered in a given status.
  if (summary['failed'] > 0 || summary['lost'] > 0) {
    return 'degraded';
  }

  if (summary['running'] > 0) {
    return 'running';
  }

  if (summary['unknown'] > 0) {
    return 'unknown';
  }

  return 'starting';
}

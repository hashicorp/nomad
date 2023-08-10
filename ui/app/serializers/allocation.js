/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

const taskGroupFromJob = (job, taskGroupName) => {
  const taskGroups = job && job.TaskGroups;
  const taskGroup =
    taskGroups && taskGroups.find((group) => group.Name === taskGroupName);
  return taskGroup ? taskGroup : null;
};

const merge = (tasks) => {
  const mergedResources = {
    Cpu: { CpuShares: 0 },
    Memory: { MemoryMB: 0 },
    Disk: { DiskMB: 0 },
  };

  return tasks.reduce((resources, task) => {
    resources.Cpu.CpuShares += (task.Cpu && task.Cpu.CpuShares) || 0;
    resources.Memory.MemoryMB += (task.Memory && task.Memory.MemoryMB) || 0;
    resources.Disk.DiskMB += (task.Disk && task.Disk.DiskMB) || 0;
    return resources;
  }, mergedResources);
};

@classic
export default class AllocationSerializer extends ApplicationSerializer {
  @service system;

  attrs = {
    taskGroupName: 'TaskGroup',
    states: 'TaskStates',
  };

  separateNanos = ['CreateTime', 'ModifyTime'];

  normalize(typeHash, hash) {
    // Transform the map-based TaskStates object into an array-based
    // TaskState fragment list
    const states = hash.TaskStates || {};
    hash.TaskStates = Object.keys(states)
      .sort()
      .map((key) => {
        const state = states[key] || {};
        const summary = { Name: key };
        Object.keys(state).forEach(
          (stateKey) => (summary[stateKey] = state[stateKey])
        );
        summary.Resources =
          hash.AllocatedResources && hash.AllocatedResources.Tasks[key];
        return summary;
      });

    hash.JobVersion =
      hash.JobVersion != null ? hash.JobVersion : get(hash, 'Job.Version');

    hash.PlainJobId = hash.JobID;
    hash.Namespace = hash.Namespace || get(hash, 'Job.Namespace') || 'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    hash.RescheduleEvents = (hash.RescheduleTracker || {}).Events;

    hash.IsMigrating = (hash.DesiredTransition || {}).Migrate;

    // API returns empty strings instead of null
    hash.PreviousAllocationID = hash.PreviousAllocation
      ? hash.PreviousAllocation
      : null;
    hash.NextAllocationID = hash.NextAllocation ? hash.NextAllocation : null;
    hash.FollowUpEvaluationID = hash.FollowupEvalID
      ? hash.FollowupEvalID
      : null;

    hash.PreemptedAllocationIDs = hash.PreemptedAllocations || [];
    hash.PreemptedByAllocationID = hash.PreemptedByAllocation || null;
    hash.WasPreempted = !!hash.PreemptedByAllocationID;

    const shared = hash.AllocatedResources && hash.AllocatedResources.Shared;
    hash.AllocatedResources =
      hash.AllocatedResources &&
      merge(Object.values(hash.AllocatedResources.Tasks));
    if (shared) {
      hash.AllocatedResources.Ports = shared.Ports;
      hash.AllocatedResources.Networks = shared.Networks;
    }

    // The Job definition for an allocation is only included in findRecord responses.
    hash.AllocationTaskGroup = !hash.Job
      ? null
      : taskGroupFromJob(hash.Job, hash.TaskGroup);

    return super.normalize(typeHash, hash);
  }
}

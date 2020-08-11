import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

const taskGroupFromJob = (job, taskGroupName) => {
  const taskGroups = job && job.TaskGroups;
  const taskGroup = taskGroups && taskGroups.find(group => group.Name === taskGroupName);
  return taskGroup ? taskGroup : null;
};

@classic
export default class AllocationSerializer extends ApplicationSerializer {
  @service system;

  attrs = {
    taskGroupName: 'TaskGroup',
    states: 'TaskStates',
  };

  mapToArray = [
    {
      name: 'TaskStates',
      convertor: (apiHash, uiHash) => {
        Object.keys(apiHash).forEach(stateKey => (uiHash[stateKey] = apiHash[stateKey]));
        uiHash.Resources = apiHash.TaskResources && apiHash.TaskResources[uiHash.Name];
      },
    },
  ];

  normalize(typeHash, hash) {
    hash.JobVersion = hash.JobVersion != null ? hash.JobVersion : get(hash, 'Job.Version');

    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    hash.ModifyTimeNanos = hash.ModifyTime % 1000000;
    hash.ModifyTime = Math.floor(hash.ModifyTime / 1000000);

    hash.CreateTimeNanos = hash.CreateTime % 1000000;
    hash.CreateTime = Math.floor(hash.CreateTime / 1000000);

    hash.RescheduleEvents = (hash.RescheduleTracker || {}).Events;

    hash.IsMigrating = (hash.DesiredTransition || {}).Migrate;

    // API returns empty strings instead of null
    hash.PreviousAllocationID = hash.PreviousAllocation ? hash.PreviousAllocation : null;
    hash.NextAllocationID = hash.NextAllocation ? hash.NextAllocation : null;
    hash.FollowUpEvaluationID = hash.FollowupEvalID ? hash.FollowupEvalID : null;

    hash.PreemptedAllocationIDs = hash.PreemptedAllocations || [];
    hash.PreemptedByAllocationID = hash.PreemptedByAllocation || null;
    hash.WasPreempted = !!hash.PreemptedByAllocationID;

    // When present, the resources are nested under AllocatedResources.Shared
    hash.AllocatedResources = hash.AllocatedResources && hash.AllocatedResources.Shared;

    // The Job definition for an allocation is only included in findRecord responses.
    hash.AllocationTaskGroup = !hash.Job ? null : taskGroupFromJob(hash.Job, hash.TaskGroup);

    return super.normalize(typeHash, hash);
  }
}

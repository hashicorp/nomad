import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  system: service(),

  attrs: {
    taskGroupName: 'TaskGroup',
    states: 'TaskStates',
  },

  normalize(typeHash, hash) {
    // Transform the map-based TaskStates object into an array-based
    // TaskState fragment list
    const states = hash.TaskStates || {};
    hash.TaskStates = Object.keys(states).map(key => {
      const state = states[key] || {};
      const summary = { Name: key };
      Object.keys(state).forEach(stateKey => (summary[stateKey] = state[stateKey]));
      summary.Resources = hash.TaskResources && hash.TaskResources[key];
      return summary;
    });

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

    // API returns empty strings instead of null
    hash.PreviousAllocationID = hash.PreviousAllocation ? hash.PreviousAllocation : null;
    hash.NextAllocationID = hash.NextAllocation ? hash.NextAllocation : null;
    hash.FollowUpEvaluationID = hash.FollowupEvalID ? hash.FollowupEvalID : null;

    return this._super(typeHash, hash);
  },
});

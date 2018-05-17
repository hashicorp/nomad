import { alias, equal, or, and } from '@ember/object/computed';
import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

const JOB_TYPES = ['service', 'batch', 'system'];

export default Model.extend({
  region: attr('string'),
  name: attr('string'),
  plainId: attr('string'),
  type: attr('string'),
  priority: attr('number'),
  allAtOnce: attr('boolean'),

  status: attr('string'),
  statusDescription: attr('string'),
  createIndex: attr('number'),
  modifyIndex: attr('number'),

  // True when the job is the parent periodic or parameterized jobs
  // Instances of periodic or parameterized jobs are false for both properties
  periodic: attr('boolean'),
  parameterized: attr('boolean'),

  periodicDetails: attr(),
  parameterizedDetails: attr(),

  hasChildren: or('periodic', 'parameterized'),

  parent: belongsTo('job', { inverse: 'children' }),
  children: hasMany('job', { inverse: 'parent' }),

  // The parent job name is prepended to child launch job names
  trimmedName: computed('name', 'parent', function() {
    return this.get('parent.content') ? this.get('name').replace(/.+?\//, '') : this.get('name');
  }),

  // A composite of type and other job attributes to determine
  // a better type descriptor for human interpretation rather
  // than for scheduling.
  displayType: computed('type', 'periodic', 'parameterized', function() {
    if (this.get('periodic')) {
      return 'periodic';
    } else if (this.get('parameterized')) {
      return 'parameterized';
    }
    return this.get('type');
  }),

  // A composite of type and other job attributes to determine
  // type for templating rather than scheduling
  templateType: computed(
    'type',
    'periodic',
    'parameterized',
    'parent.periodic',
    'parent.parameterized',
    function() {
      const type = this.get('type');

      if (this.get('periodic')) {
        return 'periodic';
      } else if (this.get('parameterized')) {
        return 'parameterized';
      } else if (this.get('parent.periodic')) {
        return 'periodic-child';
      } else if (this.get('parent.parameterized')) {
        return 'parameterized-child';
      } else if (JOB_TYPES.includes(type)) {
        // Guard against the API introducing a new type before the UI
        // is prepared to handle it.
        return this.get('type');
      }

      // A fail-safe in the event the API introduces a new type.
      return 'service';
    }
  ),

  datacenters: attr(),
  taskGroups: fragmentArray('task-group', { defaultValue: () => [] }),
  summary: belongsTo('job-summary'),

  // A job model created from the jobs list response will be lacking
  // task groups. This is an indicator that it needs to be reloaded
  // if task group information is important.
  isPartial: equal('taskGroups.length', 0),

  // If a job has only been loaded through the list request, the task groups
  // are still unknown. However, the count of task groups is available through
  // the job-summary model which is embedded in the jobs list response.
  taskGroupCount: or('taskGroups.length', 'taskGroupSummaries.length'),

  // Alias through to the summary, as if there was no relationship
  taskGroupSummaries: alias('summary.taskGroupSummaries'),
  queuedAllocs: alias('summary.queuedAllocs'),
  startingAllocs: alias('summary.startingAllocs'),
  runningAllocs: alias('summary.runningAllocs'),
  completeAllocs: alias('summary.completeAllocs'),
  failedAllocs: alias('summary.failedAllocs'),
  lostAllocs: alias('summary.lostAllocs'),
  totalAllocs: alias('summary.totalAllocs'),
  pendingChildren: alias('summary.pendingChildren'),
  runningChildren: alias('summary.runningChildren'),
  deadChildren: alias('summary.deadChildren'),
  totalChildren: alias('summary.totalChildren'),

  version: attr('number'),

  versions: hasMany('job-versions'),
  allocations: hasMany('allocations'),
  deployments: hasMany('deployments'),
  evaluations: hasMany('evaluations'),
  namespace: belongsTo('namespace'),

  hasBlockedEvaluation: computed('evaluations.@each.isBlocked', function() {
    return this.get('evaluations')
      .toArray()
      .some(evaluation => evaluation.get('isBlocked'));
  }),

  hasPlacementFailures: and('latestFailureEvaluation', 'hasBlockedEvaluation'),

  latestEvaluation: computed('evaluations.@each.modifyIndex', 'evaluations.isPending', function() {
    const evaluations = this.get('evaluations');
    if (!evaluations || evaluations.get('isPending')) {
      return null;
    }
    return evaluations.sortBy('modifyIndex').get('lastObject');
  }),

  latestFailureEvaluation: computed(
    'evaluations.@each.modifyIndex',
    'evaluations.isPending',
    function() {
      const evaluations = this.get('evaluations');
      if (!evaluations || evaluations.get('isPending')) {
        return null;
      }

      const failureEvaluations = evaluations.filterBy('hasPlacementFailures');
      if (failureEvaluations) {
        return failureEvaluations.sortBy('modifyIndex').get('lastObject');
      }
    }
  ),

  supportsDeployments: equal('type', 'service'),

  runningDeployment: computed('deployments.@each.status', function() {
    return this.get('deployments').findBy('status', 'running');
  }),

  fetchRawDefinition() {
    return this.store.adapterFor('job').fetchRawDefinition(this);
  },

  forcePeriodic() {
    return this.store.adapterFor('job').forcePeriodic(this);
  },

  stop() {
    return this.store.adapterFor('job').stop(this);
  },

  statusClass: computed('status', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      dead: 'is-light',
    };

    return classMap[this.get('status')] || 'is-dark';
  }),

  payload: attr('string'),
  decodedPayload: computed('payload', function() {
    // Lazily decode the base64 encoded payload
    return window.atob(this.get('payload') || '');
  }),
});

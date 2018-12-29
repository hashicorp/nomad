import { alias, equal, or, and } from '@ember/object/computed';
import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import RSVP from 'rsvp';
import { assert } from '@ember/debug';

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

  drivers: computed('taskGroups.@each.drivers', function() {
    return this.get('taskGroups')
      .mapBy('drivers')
      .reduce((all, drivers) => {
        all.push(...drivers);
        return all;
      }, [])
      .uniq();
  }),

  // Getting all unhealthy drivers for a job can be incredibly expensive if the job
  // has many allocations. This can lead to making an API request for many nodes.
  unhealthyDrivers: computed('allocations.@each.unhealthyDrivers.[]', function() {
    return this.get('allocations')
      .mapBy('unhealthyDrivers')
      .reduce((all, drivers) => {
        all.push(...drivers);
        return all;
      }, [])
      .uniq();
  }),

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

  latestDeployment: belongsTo('deployment', { inverse: 'jobForLatest' }),

  runningDeployment: computed('latestDeployment', 'latestDeployment.isRunning', function() {
    const latest = this.get('latestDeployment');
    if (latest.get('isRunning')) return latest;
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

  plan() {
    assert('A job must be parsed before planned', this.get('_newDefinitionJSON'));
    return this.store.adapterFor('job').plan(this);
  },

  run() {
    assert('A job must be parsed before ran', this.get('_newDefinitionJSON'));
    return this.store.adapterFor('job').run(this);
  },

  update() {
    assert('A job must be parsed before updated', this.get('_newDefinitionJSON'));
    return this.store.adapterFor('job').update(this);
  },

  parse() {
    const definition = this.get('_newDefinition');
    let promise;

    try {
      // If the definition is already JSON then it doesn't need to be parsed.
      const json = JSON.parse(definition);
      this.set('_newDefinitionJSON', json);

      // You can't set the ID of a record that already exists
      if (this.get('isNew')) {
        this.setIdByPayload(json);
      }

      promise = RSVP.resolve(definition);
    } catch (err) {
      // If the definition is invalid JSON, assume it is HCL. If it is invalid
      // in anyway, the parse endpoint will throw an error.
      promise = this.store
        .adapterFor('job')
        .parse(this.get('_newDefinition'))
        .then(response => {
          this.set('_newDefinitionJSON', response);
          this.setIdByPayload(response);
        });
    }

    return promise;
  },

  setIdByPayload(payload) {
    const namespace = payload.Namespace || 'default';
    const id = payload.Name;

    this.set('plainId', id);
    this.set('id', JSON.stringify([id, namespace]));

    const namespaceRecord = this.store.peekRecord('namespace', namespace);
    if (namespaceRecord) {
      this.set('namespace', namespaceRecord);
    }
  },

  resetId() {
    this.set('id', JSON.stringify([this.get('plainId'), this.get('namespace.name') || 'default']));
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

  // An arbitrary HCL or JSON string that is used by the serializer to plan
  // and run this job. Used for both new job models and saved job models.
  _newDefinition: attr('string'),

  // The new definition may be HCL, in which case the API will need to parse the
  // spec first. In order to preserve both the original HCL and the parsed response
  // that will be submitted to the create job endpoint, another prop is necessary.
  _newDefinitionJSON: attr('string'),
});

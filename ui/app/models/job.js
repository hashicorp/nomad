/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { alias, equal, or, and, mapBy } from '@ember/object/computed';
import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import RSVP from 'rsvp';
import { assert } from '@ember/debug';
import classic from 'ember-classic-decorator';
import { jobAllocStatuses } from '../utils/allocation-client-statuses';

const JOB_TYPES = ['service', 'batch', 'system', 'sysbatch'];

@classic
export default class Job extends Model {
  @attr('string') region;
  @attr('string') name;
  @attr('string') plainId;
  @attr('string') type;
  @attr('number') priority;
  @attr('boolean') allAtOnce;

  @attr('string') status;
  @attr('string') statusDescription;
  @attr('number') createIndex;
  @attr('number') modifyIndex;
  @attr('date') submitTime;
  @attr('string') nodePool; // Jobs are related to Node Pools either directly or via its Namespace, but no relationship.
  @attr('boolean') stopped;
  @attr() ui;

  @attr('number') groupCountSum;
  // if it's a system/sysbatch job, groupCountSum is allocs uniqued by nodeID
  get expectedRunningAllocCount() {
    if (this.type === 'system' || this.type === 'sysbatch') {
      return this.allocations.filterBy('nodeID').uniqBy('nodeID').length;
    } else {
      return this.groupCountSum;
    }
  }

  /**
   * @typedef {Object} LatestDeploymentSummary
   * @property {boolean} IsActive - Whether the deployment is currently active
   * @property {number} JobVersion - The version of the job that was deployed
   * @property {string} Status - The status of the deployment
   * @property {string} StatusDescription - A description of the deployment status
   * @property {boolean} AllAutoPromote - Whether all allocations were auto-promoted
   * @property {boolean} RequiresPromotion - Whether the deployment requires promotion
   */
  @attr() latestDeploymentSummary;

  @attr() childStatuses;

  get childStatusBreakdown() {
    // child statuses is something like ['dead', 'dead', 'complete', 'running', 'running', 'dead'].
    // Return an object counting by status, like {dead: 3, complete: 1, running: 2}

    if (!this.childStatuses) return {};

    const breakdown = {};
    this.childStatuses.forEach((status) => {
      if (breakdown[status]) {
        breakdown[status]++;
      } else {
        breakdown[status] = 1;
      }
    });
    return breakdown;
  }

  // When we detect the deletion/purge of a job from within that job page, we kick the user out to the jobs index.
  // But what about when that purge is detected from the jobs index?
  // We set this flag to true to let the user know that the job has been removed without simply nixing it from view.
  @attr('boolean', { defaultValue: false }) assumeGC;

  get allocTypes() {
    return jobAllocStatuses[this.type].map((type) => {
      return {
        label: type,
      };
    });
  }

  /**
   * @typedef {Object} CurrentStatus
   * @property {"Healthy"|"Failed"|"Deploying"|"Degraded"|"Recovering"|"Complete"|"Running"|"Removed"|"Stopped"|"Scaled Down"} label - The current status of the job
   * @property {"highlight"|"success"|"warning"|"critical"|"neutral"} state -
   */

  /**
   * @typedef {Object} HealthStatus
   * @property {Array} nonCanary
   * @property {Array} canary
   */

  /**
   * @typedef {Object} AllocationStatus
   * @property {HealthStatus} healthy
   * @property {HealthStatus} unhealthy
   * @property {HealthStatus} health unknown
   */

  /**
   * @typedef {Object} AllocationBlock
   * @property {AllocationStatus} [running]
   * @property {AllocationStatus} [pending]
   * @property {AllocationStatus} [failed]
   * @property {AllocationStatus} [lost]
   * @property {AllocationStatus} [unplaced]
   * @property {AllocationStatus} [complete]
   */

  /**
   * Looks through running/pending allocations with the aim of filling up your desired number of allocations.
   * If any desired remain, it will walk backwards through job versions and other allocation types to build
   * a picture of the job's overall status.
   *
   * @returns {AllocationBlock} An object containing healthy non-canary allocations
   *                            for each clientStatus.
   */
  get allocBlocks() {
    let availableSlotsToFill = this.expectedRunningAllocCount;

    // Initialize allocationsOfShowableType with empty arrays for each clientStatus
    /**
     * @type {AllocationBlock}
     */
    let allocationsOfShowableType = this.allocTypes.reduce(
      (accumulator, type) => {
        accumulator[type.label] = { healthy: { nonCanary: [] } };
        return accumulator;
      },
      {}
    );

    // First accumulate the Running/Pending allocations
    for (const alloc of this.allocations.filter(
      (a) => a.clientStatus === 'running' || a.clientStatus === 'pending'
    )) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
      availableSlotsToFill--;
    }
    // TODO: return early here if !availableSlotsToFill
    // Sort all allocs by jobVersion in descending order
    const sortedAllocs = this.allocations
      .filter(
        (a) => a.clientStatus !== 'running' && a.clientStatus !== 'pending'
      )
      .sort((a, b) => {
        // First sort by jobVersion
        if (a.jobVersion > b.jobVersion) return 1;
        if (a.jobVersion < b.jobVersion) return -1;

        // If jobVersion is the same, sort by status order
        // For example, we may have some allocBlock slots to fill, and need to determine
        // if the user expects to see, from non-running/non-pending allocs, some old "failed" ones
        // or "lost" or "complete" ones, etc. jobAllocStatuses give us this order.
        if (a.jobVersion === b.jobVersion) {
          return (
            jobAllocStatuses[this.type].indexOf(b.clientStatus) -
            jobAllocStatuses[this.type].indexOf(a.clientStatus)
          );
        } else {
          return 0;
        }
      })
      .reverse();

    // Iterate over the sorted allocs
    for (const alloc of sortedAllocs) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      // If the alloc has another clientStatus, add it to the corresponding list
      // as long as we haven't reached the expectedRunningAllocCount limit for that clientStatus
      if (
        this.allocTypes.map(({ label }) => label).includes(status) &&
        allocationsOfShowableType[status].healthy.nonCanary.length <
          this.expectedRunningAllocCount
      ) {
        allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
        availableSlotsToFill--;
      }
    }

    // Handle unplaced allocs
    if (availableSlotsToFill > 0) {
      // TODO: JSDoc types for unhealty and health unknown aren't optional, but should be.
      allocationsOfShowableType['unplaced'] = {
        healthy: {
          nonCanary: Array(availableSlotsToFill)
            .fill()
            .map(() => {
              return { clientStatus: 'unplaced' };
            }),
        },
      };
    }

    return allocationsOfShowableType;
  }

  /**
   * A single status to indicate how a job is doing, based on running/healthy allocations vs desired.
   * Possible statuses are:
   * - Deploying: A deployment is actively taking place
   * - Complete: (Batch/Sysbatch only) All expected allocations are complete
   * - Running: (Batch/Sysbatch only) All expected allocations are running
   * - Healthy: All expected allocations are running and healthy
   * - Recovering: Some allocations are pending
   * - Degraded: A deployment is not taking place, and some allocations are failed, lost, or unplaced
   * - Failed: All allocations are failed, lost, or unplaced
   * - Removed: The job appeared in our initial query, but has since been garbage collected
   * - Stopped: The job has been manually stopped (and not purged or yet garbage collected) by a user
   * - Scaled Down: The job is intentionally scaled down to 0 desired allocations (all task groups have count=0)
   * @returns {CurrentStatus}
   */
  /**
   * A general assessment for how a job is going, in a non-deployment state
   * @returns {CurrentStatus}
   */
  get aggregateAllocStatus() {
    let totalAllocs = this.expectedRunningAllocCount;

    // If deploying:
    if (this.latestDeploymentSummary?.IsActive) {
      return { label: 'Deploying', state: 'highlight' };
    }

    // if manually stopped by a user:
    if (this.status === 'dead' && this.stopped) {
      return {
        label: 'Stopped',
        state: 'neutral',
      };
    }

    // If the job is scaled down to 0 desired allocations, we shouldn't call it "failed";
    // we should indicate that it is deliberately set to not have any running parts.
    if (totalAllocs === 0) {
      return { label: 'Scaled Down', state: 'neutral' };
    }

    // If the job was requested initially, but a subsequent request for it was
    // not found, we can remove links to it but maintain its presence in the list
    // until the user specifies they want a refresh
    if (this.assumeGC) {
      return { label: 'Removed', state: 'neutral' };
    }

    if (this.type === 'batch' || this.type === 'sysbatch') {
      // TODO: showing as failed when long-complete
      // If all the allocs are complete, the job is Complete
      const completeAllocs = this.allocBlocks.complete?.healthy?.nonCanary;
      if (completeAllocs?.length === totalAllocs) {
        return { label: 'Complete', state: 'success' };
      }

      // If any allocations are running the job is "Running"
      const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
      if (healthyAllocs?.length + completeAllocs?.length === totalAllocs) {
        return { label: 'Running', state: 'success' };
      }
    }

    // All the exepected allocs are running and healthy? Congratulations!
    const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
    if (totalAllocs && healthyAllocs?.length === totalAllocs) {
      return { label: 'Healthy', state: 'success' };
    }

    // If any allocations are pending the job is "Recovering"
    // Note: Batch/System jobs (which do not have deployments)
    // go into "recovering" right away, since some of their statuses are
    // "pending" as they come online. This feels a little wrong but it's kind
    // of correct?
    const pendingAllocs = this.allocBlocks.pending?.healthy?.nonCanary;
    if (pendingAllocs?.length > 0) {
      return { label: 'Recovering', state: 'highlight' };
    }

    // If any allocations are failed, lost, or unplaced in a steady state,
    // the job is "Degraded"
    const failedOrLostAllocs = [
      ...this.allocBlocks.failed?.healthy?.nonCanary,
      ...this.allocBlocks.lost?.healthy?.nonCanary,
      ...this.allocBlocks.unplaced?.healthy?.nonCanary,
    ];

    if (failedOrLostAllocs.length >= totalAllocs) {
      return { label: 'Failed', state: 'critical' };
    } else {
      return { label: 'Degraded', state: 'warning' };
    }
  }
  @fragment('structured-attributes') meta;

  @attr('boolean') isPack;

  /**
   * A task with a schedule block can have execution paused at specific cron-based times.
   * If one is currently paused, an allocation at /statuses will come back with hasPausedTask=true.
   * We should represent this to the user in the job row.
   */
  get hasPausedTask() {
    if (!this.allocations) {
      return false;
    }
    return this.allocations.any((alloc) => alloc.hasPausedTask);
  }

  // True when the job is the parent periodic or parameterized jobs
  // Instances of periodic or parameterized jobs are false for both properties
  @attr('boolean') periodic;
  @attr('boolean') parameterized;
  @attr('boolean') dispatched;

  @attr() periodicDetails;
  @attr() parameterizedDetails;

  @computed('plainId')
  get idWithNamespace() {
    return `${this.plainId}@${this.belongsTo('namespace').id() ?? 'default'}`;
  }

  @computed('periodic', 'parameterized', 'dispatched')
  get hasChildren() {
    return this.periodic || (this.parameterized && !this.dispatched);
  }

  @computed('type')
  get hasClientStatus() {
    return this.type === 'system' || this.type === 'sysbatch';
  }

  @belongsTo('job', { inverse: 'children' }) parent;
  @hasMany('job', { inverse: 'parent' }) children;

  // The parent job name is prepended to child launch job names
  @computed('name', 'parent.content')
  get trimmedName() {
    return this.get('parent.content')
      ? this.name.replace(/.+?\//, '')
      : this.name;
  }

  // A composite of type and other job attributes to determine
  // a better type descriptor for human interpretation rather
  // than for scheduling.
  @computed('isPack', 'type', 'periodic', 'parameterized')
  get displayType() {
    if (this.periodic) {
      return { type: 'periodic', isPack: this.isPack };
    } else if (this.parameterized) {
      return { type: 'parameterized', isPack: this.isPack };
    }
    return { type: this.type, isPack: this.isPack };
  }

  // A composite of type and other job attributes to determine
  // type for templating rather than scheduling
  @computed(
    'type',
    'periodic',
    'parameterized',
    'parent.{periodic,parameterized}'
  )
  get templateType() {
    const type = this.type;

    if (this.get('parent.periodic')) {
      return 'periodic-child';
    } else if (this.get('parent.parameterized')) {
      return 'parameterized-child';
    } else if (this.periodic) {
      return 'periodic';
    } else if (this.parameterized) {
      return 'parameterized';
    } else if (JOB_TYPES.includes(type)) {
      // Guard against the API introducing a new type before the UI
      // is prepared to handle it.
      return this.type;
    }

    // A fail-safe in the event the API introduces a new type.
    return 'service';
  }

  @attr() datacenters;
  @fragmentArray('task-group', { defaultValue: () => [] }) taskGroups;
  @belongsTo('job-summary') summary;

  // A job model created from the jobs list response will be lacking
  // task groups. This is an indicator that it needs to be reloaded
  // if task group information is important.
  @equal('taskGroups.length', 0) isPartial;

  // If a job has only been loaded through the list request, the task groups
  // are still unknown. However, the count of task groups is available through
  // the job-summary model which is embedded in the jobs list response.
  @or('taskGroups.length', 'taskGroupSummaries.length') taskGroupCount;

  // Alias through to the summary, as if there was no relationship
  @alias('summary.taskGroupSummaries') taskGroupSummaries;
  @alias('summary.queuedAllocs') queuedAllocs;
  @alias('summary.startingAllocs') startingAllocs;
  @alias('summary.runningAllocs') runningAllocs;
  @alias('summary.completeAllocs') completeAllocs;
  @alias('summary.failedAllocs') failedAllocs;
  @alias('summary.lostAllocs') lostAllocs;
  @alias('summary.unknownAllocs') unknownAllocs;
  @alias('summary.totalAllocs') totalAllocs;
  @alias('summary.pendingChildren') pendingChildren;
  @alias('summary.runningChildren') runningChildren;
  @alias('summary.deadChildren') deadChildren;
  @alias('summary.totalChildren') totalChildren;

  @attr('number') version;

  @hasMany('job-versions', { async: true }) versions;
  @hasMany('allocations') allocations;
  @hasMany('deployments') deployments;
  @hasMany('evaluations') evaluations;
  @hasMany('variables') variables;
  @belongsTo('namespace') namespace;
  @belongsTo('job-scale') scaleState;
  @hasMany('services') services;

  @hasMany('recommendation-summary') recommendationSummaries;

  get actions() {
    return this.taskGroups.reduce((acc, taskGroup) => {
      return acc.concat(
        taskGroup.tasks
          .map((task) => {
            return task.get('actions')?.toArray() || [];
          })
          .reduce((taskAcc, taskActions) => taskAcc.concat(taskActions), [])
      );
    }, []);
  }

  /**
   *
   * @param {import('../models/action').default} action
   * @param {string} allocID
   * @param {import('../models/action-instance').default} actionInstance
   * @returns
   */
  getActionSocketUrl(action, allocID, actionInstance) {
    return this.store
      .adapterFor('job')
      .getActionSocketUrl(this, action, allocID, actionInstance);
  }

  @computed('taskGroups.@each.drivers')
  get drivers() {
    return this.taskGroups
      .mapBy('drivers')
      .reduce((all, drivers) => {
        all.push(...drivers);
        return all;
      }, [])
      .uniq();
  }

  @mapBy('allocations', 'unhealthyDrivers') allocationsUnhealthyDrivers;

  // Getting all unhealthy drivers for a job can be incredibly expensive if the job
  // has many allocations. This can lead to making an API request for many nodes.
  @computed('allocations', 'allocationsUnhealthyDrivers.[]')
  get unhealthyDrivers() {
    return this.allocations
      .mapBy('unhealthyDrivers')
      .reduce((all, drivers) => {
        all.push(...drivers);
        return all;
      }, [])
      .uniq();
  }

  @computed('evaluations.@each.isBlocked')
  get hasBlockedEvaluation() {
    return this.evaluations
      .toArray()
      .some((evaluation) => evaluation.get('isBlocked'));
  }

  @and('latestFailureEvaluation', 'hasBlockedEvaluation') hasPlacementFailures;

  @computed('evaluations.{@each.modifyIndex,isPending}')
  get latestEvaluation() {
    const evaluations = this.evaluations;
    if (!evaluations || evaluations.get('isPending')) {
      return null;
    }
    return evaluations.sortBy('modifyIndex').get('lastObject');
  }

  @computed('evaluations.{@each.modifyIndex,isPending}')
  get latestFailureEvaluation() {
    const evaluations = this.evaluations;
    if (!evaluations || evaluations.get('isPending')) {
      return null;
    }

    const failureEvaluations = evaluations.filterBy('hasPlacementFailures');
    if (failureEvaluations) {
      return failureEvaluations.sortBy('modifyIndex').get('lastObject');
    }

    return undefined;
  }

  @equal('type', 'service') supportsDeployments;

  @belongsTo('deployment', { inverse: 'jobForLatest' }) latestDeployment;

  @computed('latestDeployment', 'latestDeployment.isRunning')
  get runningDeployment() {
    const latest = this.latestDeployment;
    if (latest.get('isRunning')) return latest;
    return undefined;
  }

  fetchRawDefinition() {
    return this.store.adapterFor('job').fetchRawDefinition(this);
  }

  fetchRawSpecification() {
    return this.store.adapterFor('job').fetchRawSpecification(this);
  }

  forcePeriodic() {
    return this.store.adapterFor('job').forcePeriodic(this);
  }

  stop() {
    return this.store.adapterFor('job').stop(this);
  }

  purge() {
    return this.store.adapterFor('job').purge(this);
  }

  plan() {
    assert('A job must be parsed before planned', this._newDefinitionJSON);
    return this.store.adapterFor('job').plan(this);
  }

  run() {
    assert('A job must be parsed before ran', this._newDefinitionJSON);
    return this.store.adapterFor('job').run(this);
  }

  update() {
    assert('A job must be parsed before updated', this._newDefinitionJSON);

    return this.store.adapterFor('job').update(this);
  }

  getVersions(diffVersion) {
    return this.store.adapterFor('job').getVersions(this, diffVersion);
  }
  parse() {
    const definition = this._newDefinition;
    const variables = this._newDefinitionVariables;
    let promise;

    try {
      // If the definition is already JSON then it doesn't need to be parsed.
      const json = JSON.parse(definition);
      this.set('_newDefinitionJSON', json);

      // You can't set the ID of a record that already exists
      if (this.isNew) {
        this.setIdByPayload(json);
      }

      promise = RSVP.resolve(definition);
    } catch (err) {
      // If the definition is invalid JSON, assume it is HCL. If it is invalid
      // in anyway, the parse endpoint will throw an error.

      promise = this.store
        .adapterFor('job')
        .parse(this._newDefinition, variables)
        .then((response) => {
          this.set('_newDefinitionJSON', response);
          this.setIdByPayload(response);
        });
    }

    return promise;
  }

  scale(group, count, message) {
    if (message == null)
      message = `Manually scaled to ${count} from the Nomad UI`;
    return this.store.adapterFor('job').scale(this, group, count, message);
  }

  dispatch(meta, payload) {
    return this.store.adapterFor('job').dispatch(this, meta, payload);
  }

  setIdByPayload(payload) {
    const namespace = payload.Namespace || 'default';
    const id = payload.Name;

    this.set('plainId', id);
    this.set('_idBeforeSaving', JSON.stringify([id, namespace]));

    const namespaceRecord = this.store.peekRecord('namespace', namespace);
    if (namespaceRecord) {
      this.set('namespace', namespaceRecord);
    }
  }

  resetId() {
    this.set(
      'id',
      JSON.stringify([this.plainId, this.get('namespace.name') || 'default'])
    );
  }

  @computed('status')
  get statusClass() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      dead: 'is-light',
    };

    return classMap[this.status] || 'is-dark';
  }

  @attr('string') payload;

  @computed('payload')
  get decodedPayload() {
    // Lazily decode the base64 encoded payload
    return window.atob(this.payload || '');
  }

  // An arbitrary HCL or JSON string that is used by the serializer to plan
  // and run this job. Used for both new job models and saved job models.
  @attr('string') _newDefinition;

  // An arbitrary JSON string that is used by the adapter to plan
  // and run this job. Used for both new job models and saved job models.
  @attr('string') _newDefinitionVariables;

  // The new definition may be HCL, in which case the API will need to parse the
  // spec first. In order to preserve both the original HCL and the parsed response
  // that will be submitted to the create job endpoint, another prop is necessary.
  @attr('string') _newDefinitionJSON;

  @computed('variables.[]', 'parent', 'plainId')
  get pathLinkedVariable() {
    if (this.parent.get('id')) {
      return this.variables?.findBy(
        'path',
        `nomad/jobs/${JSON.parse(this.parent.get('id'))[0]}`
      );
    } else {
      return this.variables?.findBy('path', `nomad/jobs/${this.plainId}`);
    }
  }

  // TODO: This async fetcher seems like a better fit for most of our use-cases than the above getter (which cannot do async/await)
  async getPathLinkedVariable() {
    await this.variables;
    if (this.parent.get('id')) {
      return this.variables?.findBy(
        'path',
        `nomad/jobs/${JSON.parse(this.parent.get('id'))[0]}`
      );
    } else {
      return this.variables?.findBy('path', `nomad/jobs/${this.plainId}`);
    }
  }
}

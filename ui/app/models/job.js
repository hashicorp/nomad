/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias, equal, or, and, mapBy } from '@ember/object/computed';
import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import RSVP from 'rsvp';
import { assert } from '@ember/debug';
import classic from 'ember-classic-decorator';

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

  @fragment('structured-attributes') meta;

  get isPack() {
    return !!this.meta?.structured?.pack;
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

  @hasMany('job-versions') versions;
  @hasMany('allocations') allocations;
  @hasMany('deployments') deployments;
  @hasMany('evaluations') evaluations;
  @hasMany('variables') variables;
  @belongsTo('namespace') namespace;
  @belongsTo('job-scale') scaleState;
  @hasMany('services') services;

  @hasMany('recommendation-summary') recommendationSummaries;

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

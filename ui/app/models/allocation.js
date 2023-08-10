/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { equal, none } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import isEqual from 'lodash.isequal';
import intersection from 'lodash.intersection';
import shortUUIDProperty from '../utils/properties/short-uuid';
import classic from 'ember-classic-decorator';

const STATUS_ORDER = {
  pending: 1,
  running: 2,
  complete: 3,
  unknown: 4,
  failed: 5,
  lost: 6,
};

@classic
export default class Allocation extends Model {
  @service token;
  @service store;

  @shortUUIDProperty('id') shortId;
  @belongsTo('job') job;
  @belongsTo('node') node;
  @attr('string') namespace;
  @attr('string') nodeID;
  @attr('string') name;
  @attr('string') taskGroupName;
  @fragment('resources') resources;
  @fragment('resources') allocatedResources;
  @attr('number') jobVersion;

  @attr('number') modifyIndex;
  @attr('date') modifyTime;

  @attr('number') createIndex;
  @attr('date') createTime;

  @attr('string') clientStatus;
  @attr('string') desiredStatus;
  @attr() desiredTransition;
  @attr() deploymentStatus;

  get isCanary() {
    return this.deploymentStatus?.Canary;
  }

  // deploymentStatus.Healthy can be true, false, or null. Null implies pending
  get isHealthy() {
    return this.deploymentStatus?.Healthy;
  }

  get isUnhealthy() {
    return this.deploymentStatus?.Healthy === false;
  }

  get willNotRestart() {
    return this.clientStatus === 'failed' || this.clientStatus === 'lost';
  }

  get willNotReschedule() {
    return (
      this.willNotRestart &&
      !this.get('nextAllocation.content') &&
      !this.get('followUpEvaluation.content')
    );
  }

  get hasBeenRescheduled() {
    return this.get('followUpEvaluation.content');
  }

  get hasBeenRestarted() {
    return this.states
      .map((s) => s.events.content)
      .flat()
      .find((e) => e.type === 'Restarting');
  }

  @attr healthChecks;

  async getServiceHealth() {
    const data = await this.store.adapterFor('allocation').check(this);

    // Compare Results
    if (!isEqual(this.healthChecks, data)) {
      this.set('healthChecks', data);
    }
  }

  @computed('')
  get plainJobId() {
    return JSON.parse(this.belongsTo('job').id())[0];
  }

  @computed('clientStatus')
  get statusIndex() {
    return STATUS_ORDER[this.clientStatus] || 100;
  }

  @equal('clientStatus', 'running') isRunning;
  @attr('boolean') isMigrating;

  @computed('clientStatus')
  get isScheduled() {
    return ['pending', 'running'].includes(this.clientStatus);
  }

  // An allocation model created from any allocation list response will be lacking
  // many properties (some of which can always be null). This is an indicator that
  // the allocation needs to be reloaded to get the complete allocation state.
  @none('allocationTaskGroup') isPartial;

  // When allocations are server-side rescheduled, a paper trail
  // is left linking all reschedule attempts.
  @belongsTo('allocation', { inverse: 'nextAllocation' }) previousAllocation;
  @belongsTo('allocation', { inverse: 'previousAllocation' }) nextAllocation;

  @hasMany('allocation', { inverse: 'preemptedByAllocation' })
  preemptedAllocations;
  @belongsTo('allocation', { inverse: 'preemptedAllocations' })
  preemptedByAllocation;
  @attr('boolean') wasPreempted;

  @belongsTo('evaluation') followUpEvaluation;

  @computed('clientStatus')
  get statusClass() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      complete: 'is-complete',
      failed: 'is-error',
      lost: 'is-light',
      unknown: 'is-unknown',
    };

    return classMap[this.clientStatus] || 'is-dark';
  }

  @computed('jobVersion', 'job.version')
  get isOld() {
    return this.jobVersion !== this.get('job.version');
  }

  @computed('isOld', 'jobTaskGroup', 'allocationTaskGroup')
  get taskGroup() {
    if (!this.isOld) return this.jobTaskGroup;
    return this.allocationTaskGroup;
  }

  @computed('taskGroupName', 'job.taskGroups.[]')
  get jobTaskGroup() {
    const taskGroups = this.get('job.taskGroups');
    return taskGroups && taskGroups.findBy('name', this.taskGroupName);
  }

  @fragment('task-group', { defaultValue: null }) allocationTaskGroup;

  @computed('taskGroup.drivers.[]', 'node.unhealthyDriverNames.[]')
  get unhealthyDrivers() {
    const taskGroupUnhealthyDrivers = this.get('taskGroup.drivers');
    const nodeUnhealthyDrivers = this.get('node.unhealthyDriverNames');

    if (taskGroupUnhealthyDrivers && nodeUnhealthyDrivers) {
      return intersection(taskGroupUnhealthyDrivers, nodeUnhealthyDrivers);
    }

    return [];
  }

  // When per_alloc is set to true on a volume, the volumes are duplicated between active allocations.
  // We differentiate them with a [#] suffix, inferred from a volume's allocation's name property.
  @computed('name')
  get volumeExtension() {
    return this.name.substring(this.name.lastIndexOf('['));
  }

  @fragmentArray('task-state') states;
  @fragmentArray('reschedule-event') rescheduleEvents;

  @computed('rescheduleEvents.length', 'nextAllocation')
  get hasRescheduleEvents() {
    return this.get('rescheduleEvents.length') > 0 || this.nextAllocation;
  }

  @computed(
    'clientStatus',
    'followUpEvaluation.content',
    'nextAllocation.content'
  )
  get hasStoppedRescheduling() {
    return (
      !this.get('nextAllocation.content') &&
      !this.get('followUpEvaluation.content') &&
      this.clientStatus === 'failed'
    );
  }

  stop() {
    return this.store.adapterFor('allocation').stop(this);
  }

  restart(taskName) {
    return this.store.adapterFor('allocation').restart(this, taskName);
  }

  restartAll() {
    return this.store.adapterFor('allocation').restartAll(this);
  }

  ls(path) {
    return this.store.adapterFor('allocation').ls(this, path);
  }

  stat(path) {
    return this.store.adapterFor('allocation').stat(this, path);
  }
}

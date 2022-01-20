import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { equal, none } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import intersection from 'lodash.intersection';
import shortUUIDProperty from '../utils/properties/short-uuid';
import classic from 'ember-classic-decorator';

const STATUS_ORDER = {
  pending: 1,
  running: 2,
  complete: 3,
  failed: 4,
  lost: 5,
};

@classic
export default class Allocation extends Model {
  @service token;

  @shortUUIDProperty('id') shortId;
  @belongsTo('job') job;
  @belongsTo('node') node;
  @attr('string') namespace;
  @attr('string') name;
  @attr('string') taskGroupName;
  @fragment('resources') resources;
  @fragment('resources') allocatedResources;
  @attr('number') jobVersion;

  // Store basic node information returned by the API to avoid the need for
  // node:read ACL permission.
  @attr('string') nodeName;
  @computed
  get shortNodeId() {
    return this.belongsTo('node').id().split('-')[0];
  }

  @attr('number') modifyIndex;
  @attr('date') modifyTime;

  @attr('number') createIndex;
  @attr('date') createTime;

  @attr('string') clientStatus;
  @attr('string') desiredStatus;

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

  ls(path) {
    return this.store.adapterFor('allocation').ls(this, path);
  }

  stat(path) {
    return this.store.adapterFor('allocation').stat(this, path);
  }
}

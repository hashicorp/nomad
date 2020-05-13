import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { equal, none } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import intersection from 'lodash.intersection';
import shortUUIDProperty from '../utils/properties/short-uuid';

const STATUS_ORDER = {
  pending: 1,
  running: 2,
  complete: 3,
  failed: 4,
  lost: 5,
};

export default Model.extend({
  token: service(),

  shortId: shortUUIDProperty('id'),
  job: belongsTo('job'),
  node: belongsTo('node'),
  name: attr('string'),
  taskGroupName: attr('string'),
  resources: fragment('resources'),
  allocatedResources: fragment('resources'),
  jobVersion: attr('number'),

  modifyIndex: attr('number'),
  modifyTime: attr('date'),

  createIndex: attr('number'),
  createTime: attr('date'),

  clientStatus: attr('string'),
  desiredStatus: attr('string'),
  statusIndex: computed('clientStatus', function() {
    return STATUS_ORDER[this.clientStatus] || 100;
  }),

  isRunning: equal('clientStatus', 'running'),
  isMigrating: attr('boolean'),

  // An allocation model created from any allocation list response will be lacking
  // many properties (some of which can always be null). This is an indicator that
  // the allocation needs to be reloaded to get the complete allocation state.
  isPartial: none('allocationTaskGroup'),

  // When allocations are server-side rescheduled, a paper trail
  // is left linking all reschedule attempts.
  previousAllocation: belongsTo('allocation', { inverse: 'nextAllocation' }),
  nextAllocation: belongsTo('allocation', { inverse: 'previousAllocation' }),

  preemptedAllocations: hasMany('allocation', { inverse: 'preemptedByAllocation' }),
  preemptedByAllocation: belongsTo('allocation', { inverse: 'preemptedAllocations' }),
  wasPreempted: attr('boolean'),

  followUpEvaluation: belongsTo('evaluation'),

  statusClass: computed('clientStatus', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      complete: 'is-complete',
      failed: 'is-error',
      lost: 'is-light',
    };

    return classMap[this.clientStatus] || 'is-dark';
  }),

  isOld: computed('jobVersion', 'job.version', function() {
    return this.jobVersion !== this.get('job.version');
  }),

  taskGroup: computed('isOld', 'jobTaskGroup', 'allocationTaskGroup', function() {
    if (!this.isOld) return this.jobTaskGroup;
    return this.allocationTaskGroup;
  }),

  jobTaskGroup: computed('taskGroupName', 'job.taskGroups.[]', function() {
    const taskGroups = this.get('job.taskGroups');
    return taskGroups && taskGroups.findBy('name', this.taskGroupName);
  }),

  allocationTaskGroup: fragment('task-group', { defaultValue: null }),

  unhealthyDrivers: computed('taskGroup.drivers.[]', 'node.unhealthyDriverNames.[]', function() {
    const taskGroupUnhealthyDrivers = this.get('taskGroup.drivers');
    const nodeUnhealthyDrivers = this.get('node.unhealthyDriverNames');

    if (taskGroupUnhealthyDrivers && nodeUnhealthyDrivers) {
      return intersection(taskGroupUnhealthyDrivers, nodeUnhealthyDrivers);
    }

    return [];
  }),

  states: fragmentArray('task-state'),
  rescheduleEvents: fragmentArray('reschedule-event'),

  hasRescheduleEvents: computed('rescheduleEvents.length', 'nextAllocation', function() {
    return this.get('rescheduleEvents.length') > 0 || this.nextAllocation;
  }),

  hasStoppedRescheduling: computed(
    'nextAllocation',
    'clientStatus',
    'followUpEvaluation.content',
    function() {
      return (
        !this.get('nextAllocation.content') &&
        !this.get('followUpEvaluation.content') &&
        this.clientStatus === 'failed'
      );
    }
  ),

  stop() {
    return this.store.adapterFor('allocation').stop(this);
  },

  restart(taskName) {
    return this.store.adapterFor('allocation').restart(this, taskName);
  },
});

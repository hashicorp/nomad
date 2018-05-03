import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import AllocationStats from '../utils/classes/allocation-stats';

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
  modifyIndex: attr('number'),
  modifyTime: attr('date'),
  jobVersion: attr('number'),

  // TEMPORARY: https://github.com/emberjs/data/issues/5209
  originalJobId: attr('string'),

  clientStatus: attr('string'),
  desiredStatus: attr('string'),
  statusIndex: computed('clientStatus', function() {
    return STATUS_ORDER[this.get('clientStatus')] || 100;
  }),

  // When allocations are server-side rescheduled, a paper trail
  // is left linking all reschedule attempts.
  previousAllocation: belongsTo('allocation', { inverse: 'nextAllocation' }),
  nextAllocation: belongsTo('allocation', { inverse: 'previousAllocation' }),

  followUpEvaluation: belongsTo('evaluation'),

  statusClass: computed('clientStatus', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      complete: 'is-complete',
      failed: 'is-error',
      lost: 'is-light',
    };

    return classMap[this.get('clientStatus')] || 'is-dark';
  }),

  taskGroup: computed('taskGroupName', 'job.taskGroups.[]', function() {
    const taskGroups = this.get('job.taskGroups');
    return taskGroups && taskGroups.findBy('name', this.get('taskGroupName'));
  }),

  fetchStats() {
    return this.get('token')
      .authorizedRequest(`/v1/client/allocation/${this.get('id')}/stats`)
      .then(res => res.json())
      .then(json => {
        return new AllocationStats({
          stats: json,
          allocation: this,
        });
      });
  },

  states: fragmentArray('task-state'),
  rescheduleEvents: fragmentArray('reschedule-event'),

  hasRescheduleEvents: computed('rescheduleEvents.length', 'nextAllocation', function() {
    return this.get('rescheduleEvents.length') > 0 || this.get('nextAllocation');
  }),

  hasStoppedRescheduling: computed(
    'nextAllocation',
    'clientStatus',
    'followUpEvaluation',
    function() {
      return (
        !this.get('nextAllocation.content') &&
        !this.get('followUpEvaluation.content') &&
        this.get('clientStatus') === 'failed'
      );
    }
  ),
});

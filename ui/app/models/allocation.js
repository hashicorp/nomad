import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import PromiseObject from '../utils/classes/promise-object';
import timeout from '../utils/timeout';
import shortUUIDProperty from '../utils/properties/short-uuid';

const { computed, RSVP, inject } = Ember;

const STATUS_ORDER = {
  pending: 1,
  running: 2,
  complete: 3,
  failed: 4,
  lost: 5,
};

export default Model.extend({
  token: inject.service(),

  shortId: shortUUIDProperty('id'),
  job: belongsTo('job'),
  node: belongsTo('node'),
  name: attr('string'),
  taskGroupName: attr('string'),
  resources: fragment('resources'),
  modifyIndex: attr('number'),
  jobVersion: attr('number'),

  // TEMPORARY: https://github.com/emberjs/data/issues/5209
  originalJobId: attr('string'),

  clientStatus: attr('string'),
  desiredStatus: attr('string'),
  statusIndex: computed('clientStatus', function() {
    return STATUS_ORDER[this.get('clientStatus')] || 100;
  }),

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

  memoryUsed: computed.readOnly('stats.ResourceUsage.MemoryStats.RSS'),
  cpuUsed: computed('stats.ResourceUsage.CpuStats.TotalTicks', function() {
    return Math.floor(this.get('stats.ResourceUsage.CpuStats.TotalTicks') || 0);
  }),

  percentMemory: computed('taskGroup.reservedMemory', 'memoryUsed', function() {
    const used = this.get('memoryUsed') / 1024 / 1024;
    const total = this.get('taskGroup.reservedMemory');
    if (!total || !used) {
      return 0;
    }
    return used / total;
  }),

  percentCPU: computed('cpuUsed', 'taskGroup.reservedCPU', function() {
    const used = this.get('cpuUsed');
    const total = this.get('taskGroup.reservedCPU');
    if (!total || !used) {
      return 0;
    }
    return used / total;
  }),

  stats: computed('node.{isPartial,httpAddr}', function() {
    const nodeIsPartial = this.get('node.isPartial');

    // If the node doesn't have an httpAddr, it's a partial record.
    // Once it reloads, this property will be dirtied and stats will load.
    if (nodeIsPartial) {
      return PromiseObject.create({
        // Never resolve, so the promise object is always in a state of pending
        promise: new RSVP.Promise(() => {}),
      });
    }

    const url = `//${this.get('node.httpAddr')}/v1/client/allocation/${this.get('id')}/stats`;
    return PromiseObject.create({
      promise: RSVP.Promise.race([
        this.get('token')
          .authorizedRequest(url)
          .then(res => res.json()),
        timeout(2000),
      ]),
    });
  }),

  states: fragmentArray('task-state'),
});

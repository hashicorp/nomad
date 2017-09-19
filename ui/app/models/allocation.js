import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import fetch from 'fetch';
import PromiseObject from '../utils/classes/promise-object';
import timeout from '../utils/timeout';
import shortUUIDProperty from '../utils/properties/short-uuid';

const { computed, RSVP } = Ember;

export default Model.extend({
  shortId: shortUUIDProperty('id'),
  job: belongsTo('job'),
  node: belongsTo('node'),
  name: attr('string'),
  taskGroupName: attr('string'),
  resources: fragment('resources'),
  modifyIndex: attr('number'),

  clientStatus: attr('string'),
  desiredStatus: attr('string'),

  taskGroup: computed('taskGroupName', 'job.taskGroups.[]', function() {
    const taskGroups = this.get('job.taskGroups');
    return taskGroups && taskGroups.findBy('name', this.get('taskGroupName'));
  }),

  percentMemory: computed(
    'taskGroup.reservedMemory',
    'stats.ResourceUsage.MemoryStats.Cache',
    function() {
      const used = this.get('stats.ResourceUsage.MemoryStats.Cache');
      const total = this.get('taskGroup.reservedMemory');
      if (!total || !used) {
        return 0;
      }
      return used / total;
    }
  ),

  percentCPU: computed('stats.ResourceUsage.CpuStats.Percent', function() {
    return this.get('stats.ResourceUsage.CpuStats.Percent') || 0;
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
      promise: RSVP.Promise.race([fetch(url).then(res => res.json()), timeout(2000)]),
    });
  }),

  states: fragmentArray('task-state'),
});

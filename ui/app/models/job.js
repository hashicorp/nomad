import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';

const { computed } = Ember;

export default Model.extend({
  region: attr('string'),
  name: attr('string'),
  type: attr('string'),
  priority: attr('number'),
  allAtOnce: attr('boolean'),

  status: attr('string'),
  statusDescription: attr('string'),
  createIndex: attr('number'),
  modifyIndex: attr('number'),

  periodic: attr('boolean'),
  parameterized: attr('boolean'),

  datacenters: attr(),
  taskGroups: attr(),

  queuedAllocs: attr('number'),
  completeAllocs: attr('number'),
  failedAllocs: attr('number'),
  runningAllocs: attr('number'),
  startingAllocs: attr('number'),
  lostAllocs: attr('number'),

  allocsList: computed.collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs'
  ),

  totalAllocs: computed.sum('allocsList'),

  lostRate: computed('lostAllocs', 'totalAllocs', function() {
    return this.get('lostAllocs') / this.get('totalAllocs');
  }),

  pendingChildren: attr('number'),
  runningChildren: attr('number'),
  deadChildren: attr('number'),
});

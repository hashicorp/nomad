import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';

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
  taskGroups: fragmentArray('task-group'),
  taskGroupSummaries: fragmentArray('task-group-summary'),

  // Aggregate allocation counts across all summaries
  queuedAllocs: sumAggregation('taskGroupSummaries', 'queuedAllocs'),
  startingAllocs: sumAggregation('taskGroupSummaries', 'startingAllocs'),
  runningAllocs: sumAggregation('taskGroupSummaries', 'runningAllocs'),
  completeAllocs: sumAggregation('taskGroupSummaries', 'completeAllocs'),
  failedAllocs: sumAggregation('taskGroupSummaries', 'failedAllocs'),
  lostAllocs: sumAggregation('taskGroupSummaries', 'lostAllocs'),

  allocsList: computed.collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs'
  ),

  totalAllocs: computed.sum('allocsList'),

  pendingChildren: attr('number'),
  runningChildren: attr('number'),
  deadChildren: attr('number'),
});

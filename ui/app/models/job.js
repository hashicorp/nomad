import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

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
  queuedAllocs: allocAggregation('taskGroupSummaries', 'queuedAllocs'),
  startingAllocs: allocAggregation('taskGroupSummaries', 'startingAllocs'),
  runningAllocs: allocAggregation('taskGroupSummaries', 'runningAllocs'),
  completeAllocs: allocAggregation('taskGroupSummaries', 'completeAllocs'),
  failedAllocs: allocAggregation('taskGroupSummaries', 'failedAllocs'),
  lostAllocs: allocAggregation('taskGroupSummaries', 'lostAllocs'),

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

function allocAggregation(summariesKey, allocsKey) {
  return computed(`${summariesKey}.@each.${allocsKey}`, function() {
    return this.get(summariesKey).mapBy(allocsKey).reduce((sum, count) => sum + count, 0);
  });
}

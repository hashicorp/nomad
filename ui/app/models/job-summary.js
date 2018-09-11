import { collect, sum } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';

export default Model.extend({
  job: belongsTo('job'),

  taskGroupSummaries: fragmentArray('task-group-summary'),

  // Aggregate allocation counts across all summaries
  queuedAllocs: sumAggregation('taskGroupSummaries', 'queuedAllocs'),
  startingAllocs: sumAggregation('taskGroupSummaries', 'startingAllocs'),
  runningAllocs: sumAggregation('taskGroupSummaries', 'runningAllocs'),
  completeAllocs: sumAggregation('taskGroupSummaries', 'completeAllocs'),
  failedAllocs: sumAggregation('taskGroupSummaries', 'failedAllocs'),
  lostAllocs: sumAggregation('taskGroupSummaries', 'lostAllocs'),

  allocsList: collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs'
  ),

  totalAllocs: sum('allocsList'),

  pendingChildren: attr('number'),
  runningChildren: attr('number'),
  deadChildren: attr('number'),

  childrenList: collect('pendingChildren', 'runningChildren', 'deadChildren'),

  totalChildren: sum('childrenList'),
});

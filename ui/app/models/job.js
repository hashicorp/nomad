import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';

const { computed } = Ember;

export default Model.extend({
  region: attr('string'),
  name: attr('string'),
  plainId: attr('string'),
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
  taskGroups: fragmentArray('task-group', { defaultValue: () => [] }),
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

  versions: hasMany('job-versions'),
  allocations: hasMany('allocations'),
  deployments: hasMany('deployments'),
  namespace: belongsTo('namespace'),

  supportsDeployments: computed.equal('type', 'service'),

  runningDeployment: computed('deployments.@each.status', function() {
    return this.get('deployments').findBy('status', 'running');
  }),

  fetchRawDefinition() {
    return this.store.adapterFor('job').fetchRawDefinition(this);
  },

  statusClass: computed('status', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      dead: 'is-light',
    };

    return classMap[this.get('status')] || 'is-dark';
  }),
});

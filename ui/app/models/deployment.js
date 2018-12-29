import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import sumAggregation from '../utils/properties/sum-aggregation';

const { computed } = Ember;

export default Model.extend({
  shortId: shortUUIDProperty('id'),

  job: belongsTo('job'),
  versionNumber: attr('number'),

  // If any task group is not promoted yet requires promotion and the deployment
  // is still running, the deployment needs promotion.
  requiresPromotion: computed('taskGroupSummaries.@each.promoted', function() {
    return (
      this.get('status') === 'running' &&
      this.get('taskGroupSummaries')
        .toArray()
        .some(summary => summary.get('requiresPromotion') && !summary.get('promoted'))
    );
  }),

  status: attr('string'),
  statusDescription: attr('string'),
  taskGroupSummaries: fragmentArray('task-group-deployment-summary'),
  allocations: hasMany('allocations'),

  version: computed('versionNumber', 'job.versions.content.@each.number', function() {
    return (this.get('job.versions') || []).findBy('number', this.get('versionNumber'));
  }),

  placedCanaries: sumAggregation('taskGroupSummaries', 'placedCanaries'),
  desiredCanaries: sumAggregation('taskGroupSummaries', 'desiredCanaries'),
  desiredTotal: sumAggregation('taskGroupSummaries', 'desiredTotal'),
  placedAllocs: sumAggregation('taskGroupSummaries', 'placedAllocs'),
  healthyAllocs: sumAggregation('taskGroupSummaries', 'healthyAllocs'),
  unhealthyAllocs: sumAggregation('taskGroupSummaries', 'unhealthyAllocs'),

  statusClass: computed('status', function() {
    const classMap = {
      running: 'is-running',
      successful: 'is-primary',
      paused: 'is-light',
      failed: 'is-error',
      cancelled: 'is-cancelled',
    };

    return classMap[this.get('status')] || 'is-dark';
  }),
});

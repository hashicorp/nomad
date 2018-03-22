import { computed } from '@ember/object';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner, fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';

const maybe = arr => arr || [];

export default Fragment.extend({
  job: fragmentOwner(),

  name: attr('string'),
  count: attr('number'),

  tasks: fragmentArray('task'),

  allocations: computed('job.allocations.@each.taskGroup', function() {
    return maybe(this.get('job.allocations')).filterBy('taskGroupName', this.get('name'));
  }),

  reservedCPU: sumAggregation('tasks', 'reservedCPU'),
  reservedMemory: sumAggregation('tasks', 'reservedMemory'),
  reservedDisk: sumAggregation('tasks', 'reservedDisk'),

  reservedEphemeralDisk: attr('number'),

  placementFailures: computed('job.latestFailureEvaluation.failedTGAllocs.[]', function() {
    const placementFailures = this.get('job.latestFailureEvaluation.failedTGAllocs');
    return placementFailures && placementFailures.findBy('name', this.get('name'));
  }),

  queuedOrStartingAllocs: computed('summary.{queuedAllocs,startingAllocs}', function() {
    return this.get('summary.queuedAllocs') + this.get('summary.startingAllocs');
  }),

  summary: computed('job.taskGroupSummaries.[]', function() {
    return maybe(this.get('job.taskGroupSummaries')).findBy('name', this.get('name'));
  }),
});

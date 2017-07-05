import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner, fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';

const { computed } = Ember;

export default Fragment.extend({
  job: fragmentOwner(),

  name: attr('string'),
  count: attr('number'),

  tasks: fragmentArray('task'),

  reservedCPU: sumAggregation('tasks', 'reservedCPU'),
  reservedMemory: sumAggregation('tasks', 'reservedMemory'),
  reservedDisk: sumAggregation('tasks', 'reservedDisk'),

  summary: computed('job.taskGroupSummaries.[]', function() {
    return this.get('job.taskGroupSummaries').findBy('name', this.get('name'));
  }),
});

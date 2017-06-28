import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

export default Fragment.extend({
  job: fragmentOwner(),

  name: attr('string'),
  count: attr('number'),

  summary: computed('job.taskGroupSummaries.[]', function() {
    return this.get('job.taskGroupSummaries').findBy('name', this.get('name'));
  }),
});

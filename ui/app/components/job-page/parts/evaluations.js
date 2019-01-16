import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  job: null,

  classNames: ['boxed-section'],

  sortedEvaluations: computed('job.evaluations.@each.modifyIndex', function() {
    return (this.get('job.evaluations') || []).sortBy('modifyIndex').reverse();
  }),
});

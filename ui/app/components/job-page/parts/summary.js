import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default Component.extend({
  store: service(),

  job: null,

  // TEMPORARY: https://github.com/emberjs/data/issues/5209
  // The summary relationship can be broken under exact load
  // order. This ensures that the summary is always shown, even
  // if the summary link on the job is broken.
  summary: computed('job.summary.content', function() {
    const summary = this.get('job.summary');
    if (summary.get('content')) {
      return summary;
    }
    return this.get('store').peekRecord('job-summary', this.get('job.id'));
  }),

  classNames: ['boxed-section'],
});

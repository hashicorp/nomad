import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';

export default Component.extend({
  tagName: 'tr',
  classNames: ['job-row', 'is-interactive'],

  job: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },

  didReceiveAttrs() {
    // Reload the job in order to get detail information
    const job = this.get('job');
    if (job && !job.get('isLoading')) {
      job.reload();
    }
  },
});

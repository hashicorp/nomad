import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['job-row', 'is-interactive'],

  job: null,

  didReceiveAttrs() {
    // Reload the job in order to get detail information
    const job = this.get('job');
    if (job) {
      job.reload();
    }
  },
});

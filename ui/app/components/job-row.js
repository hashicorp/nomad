import Ember from 'ember';

const { Component, computed } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['job-row'],

  job: null,

  allocDistribution: computed(
    'job.{queuedAllocs,completeAllocs,failedAllocs,runningAllocs,startingAllocs}',
    function() {
      const allocs = this.get('job').getProperties(
        'queuedAllocs',
        'completeAllocs',
        'failedAllocs',
        'runningAllocs',
        'startingAllocs'
      );
      return [
        { label: 'Queued', value: allocs.queuedAllocs, className: 'queued' },
        {
          label: 'Starting',
          value: allocs.startingAllocs,
          className: 'starting',
        },
        { label: 'Running', value: allocs.runningAllocs, className: 'running' },
        {
          label: 'Complete',
          value: allocs.completeAllocs,
          className: 'complete',
        },
        { label: 'Failed', value: allocs.failedAllocs, className: 'failed' },
      ];
    }
  ),

  didReceiveAttrs() {
    // Reload the job in order to get detail information
    const job = this.get('job');
    if (job) {
      job.reload();
    }
  },
});

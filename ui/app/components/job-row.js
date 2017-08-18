import Ember from 'ember';

const { Component, computed } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['job-row', 'is-interactive'],

  job: null,

  statusClass: computed('job.status', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      dead: 'is-light',
    };

    return classMap[this.get('job.status')] || 'is-dark';
  }),

  allocDistribution: computed(
    'job.{queuedAllocs,completeAllocs,failedAllocs,runningAllocs,startingAllocs}',
    function() {
      const allocs = this.get('job').getProperties(
        'queuedAllocs',
        'completeAllocs',
        'failedAllocs',
        'runningAllocs',
        'startingAllocs',
        'lostAllocs'
      );
      return [
        { label: 'Queued', value: allocs.queuedAllocs, className: 'queued' },
        {
          label: 'Starting',
          value: allocs.startingAllocs,
          className: 'starting',
          layers: 2,
        },
        { label: 'Running', value: allocs.runningAllocs, className: 'running' },
        {
          label: 'Complete',
          value: allocs.completeAllocs,
          className: 'complete',
        },
        { label: 'Failed', value: allocs.failedAllocs, className: 'failed' },
        { label: 'Lost', value: allocs.lostAllocs, className: 'lost' },
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

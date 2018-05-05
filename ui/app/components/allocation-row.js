import Ember from 'ember';
import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { run } from '@ember/runloop';
import { lazyClick } from '../helpers/lazy-click';
import { task, timeout } from 'ember-concurrency';

export default Component.extend({
  store: service(),

  tagName: 'tr',

  classNames: ['allocation-row', 'is-interactive'],

  allocation: null,

  // Used to determine whether the row should mention the node or the job
  context: null,

  backoffSequence: computed(() => [500, 800, 1300, 2100, 3400, 5500]),

  // Internal state
  stats: null,
  statsError: false,

  enablePolling: computed(() => !Ember.testing),

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },

  didReceiveAttrs() {
    // TODO: Use this code again once the temporary workaround below
    // is resolved.

    // If the job for this allocation is incomplete, reload it to get
    // detailed information.
    // const allocation = this.get('allocation');
    // if (
    //   allocation &&
    //   allocation.get('job') &&
    //   !allocation.get('job.isPending') &&
    //   !allocation.get('taskGroup')
    // ) {
    //   const job = allocation.get('job.content');
    //   job && job.reload();
    // }

    // TEMPORARY: https://github.com/emberjs/data/issues/5209
    // Ember Data doesn't like it when relationships aren't reflective,
    // which means the allocation's job will be null if it hasn't been
    // resolved through the allocation (allocation.get('job')) before
    // being resolved through the store (store.findAll('job')). The
    // workaround is to persist the jobID as a string on the allocation
    // and manually re-link the two records here.
    const allocation = this.get('allocation');

    if (allocation) {
      run.scheduleOnce('afterRender', this, qualifyAllocation);
    } else {
      this.get('fetchStats').cancelAll();
      this.set('stats', null);
    }
  },

  fetchStats: task(function*(allocation) {
    const backoffSequence = this.get('backoffSequence').slice();
    const maxTiming = backoffSequence.pop();

    do {
      try {
        const stats = yield allocation.fetchStats();
        this.set('stats', stats);
        this.set('statsError', false);
      } catch (error) {
        this.set('statsError', true);
      }
      yield timeout(backoffSequence.shift() || maxTiming);
    } while (this.get('enablePolling'));
  }).drop(),
});

function qualifyAllocation() {
  const allocation = this.get('allocation');
  return allocation.reload().then(() => {
    this.get('fetchStats').perform(allocation);
    run.scheduleOnce('afterRender', this, qualifyJob);
  });
}

function qualifyJob() {
  const allocation = this.get('allocation');
  if (allocation.get('originalJobId')) {
    const job = this.get('store').peekRecord('job', allocation.get('originalJobId'));
    if (job) {
      allocation.setProperties({
        job,
        originalJobId: null,
      });
      if (job.get('isPartial')) {
        job.reload();
      }
    } else {
      this.get('store')
        .findRecord('job', allocation.get('originalJobId'))
        .then(job => {
          allocation.set('job', job);
        });
    }
  }
}

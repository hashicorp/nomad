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

    // Make sure that the job record in the store for this allocation
    // is complete and not a partial from the list endpoint
    if (
      allocation &&
      allocation.get('job') &&
      !allocation.get('job.isPending') &&
      !allocation.get('taskGroup')
    ) {
      const job = allocation.get('job.content');
      job && job.reload();
    }
  });
}

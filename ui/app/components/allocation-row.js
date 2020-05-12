import Ember from 'ember';
import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { alias } from '@ember/object/computed';
import { run } from '@ember/runloop';
import { task, timeout } from 'ember-concurrency';
import { lazyClick } from '../helpers/lazy-click';
import AllocationStatsTracker from 'nomad-ui/utils/classes/allocation-stats-tracker';

export default Component.extend({
  store: service(),
  token: service(),

  tagName: 'tr',

  classNames: ['allocation-row', 'is-interactive'],

  allocation: null,

  // Used to determine whether the row should mention the node or the job
  context: null,

  // Internal state
  statsError: false,

  enablePolling: overridable(() => !Ember.testing),

  stats: computed('allocation', 'allocation.isRunning', function() {
    if (!this.get('allocation.isRunning')) return;

    return AllocationStatsTracker.create({
      fetch: url => this.token.authorizedRequest(url),
      allocation: this.allocation,
    });
  }),

  cpu: alias('stats.cpu.lastObject'),
  memory: alias('stats.memory.lastObject'),

  onClick() {},

  click(event) {
    lazyClick([this.onClick, event]);
  },

  didReceiveAttrs() {
    this.updateStatsTracker();
  },

  updateStatsTracker() {
    const allocation = this.allocation;

    if (allocation) {
      run.scheduleOnce('afterRender', this, qualifyAllocation);
    } else {
      this.fetchStats.cancelAll();
    }
  },

  fetchStats: task(function*() {
    do {
      if (this.stats) {
        try {
          yield this.get('stats.poll').perform();
          this.set('statsError', false);
        } catch (error) {
          this.set('statsError', true);
        }
      }

      yield timeout(500);
    } while (this.enablePolling);
  }).drop(),
});

async function qualifyAllocation() {
  const allocation = this.allocation;

  // Make sure the allocation is a complete record and not a partial so we
  // can show information such as preemptions and rescheduled allocation.
  await allocation.reload();

  if (allocation.get('job.isPending')) {
    // Make sure the job is loaded before starting the stats tracker
    await allocation.get('job');
  } else if (!allocation.get('taskGroup')) {
    // Make sure that the job record in the store for this allocation
    // is complete and not a partial from the list endpoint
    const job = allocation.get('job.content');
    if (job) await job.reload();
  }

  this.fetchStats.perform();
}

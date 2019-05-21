import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed, observer } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { watchRecord } from 'nomad-ui/utils/properties/watch';

export default Controller.extend(Sortable, {
  token: service(),

  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  listToSort: alias('model.states'),
  sortedStates: alias('listSorted'),

  // Set in the route
  preempter: null,

  error: computed(() => {
    // { title, description }
    return null;
  }),

  onDismiss() {
    this.set('error', null);
  },

  watchNext: watchRecord('allocation'),

  observeWatchNext: observer('model.nextAllocation.clientStatus', function() {
    const nextAllocation = this.model.nextAllocation;
    if (nextAllocation && nextAllocation.content) {
      this.watchNext.perform(nextAllocation);
    } else {
      this.watchNext.cancelAll();
    }
  }),

  stopAllocation: task(function*() {
    try {
      yield this.model.stop();
      // Eagerly update the allocation clientStatus to avoid flickering
      this.model.set('clientStatus', 'complete');
    } catch (err) {
      this.set('error', {
        title: 'Could Not Stop Allocation',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),

  restartAllocation: task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Allocation',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),

  actions: {
    gotoTask(allocation, task) {
      this.transitionToRoute('allocations.allocation.task', task);
    },

    taskClick(allocation, task, event) {
      lazyClick([() => this.send('gotoTask', allocation, task), event]);
    },
  },
});

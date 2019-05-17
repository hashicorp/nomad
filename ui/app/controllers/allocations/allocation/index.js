import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

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

  stopAllocation: task(function*() {
    try {
      yield this.model.stop();
      // Eagerly update the allocation clientStatus to avoid flickering
      this.model.set('clientStatus', 'complete');
    } catch (err) {
      this.set('error', {
        title: 'Could Not Stop Allocation',
        description: 'Your ACL token does not grant allocation lifecyle permissions.',
      });
    }
  }),

  restartAllocation: task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      console.log('oops', err);
      this.set('error', {
        title: 'Could Not Stop Allocation',
        description: 'Your ACL token does not grant allocation lifecyle permissions.',
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

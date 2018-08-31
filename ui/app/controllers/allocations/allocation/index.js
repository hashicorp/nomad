import Ember from 'ember';
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { stats } from 'nomad-ui/utils/classes/allocation-stats-tracker';

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

  stats: stats('model', function statsFetch() {
    return url => this.get('token').authorizedRequest(url);
  }),

  pollStats: task(function*() {
    do {
      yield this.get('stats').poll();
      yield timeout(1000);
    } while (!Ember.testing);
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

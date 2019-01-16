import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default Controller.extend(Sortable, {
  allocationController: controller('allocations.allocation'),

  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  breadcrumbs: alias('allocationController.breadcrumbs'),

  listToSort: alias('model.states'),
  sortedStates: alias('listSorted'),

  actions: {
    gotoTask(allocation, task) {
      this.transitionToRoute('allocations.allocation.task', task);
    },

    taskClick(allocation, task, event) {
      lazyClick([() => this.send('gotoTask', allocation, task), event]);
    },
  },
});

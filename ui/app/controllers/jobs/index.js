import { inject as service } from '@ember/service';
import { alias, filterBy } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

export default Controller.extend(Sortable, Searchable, {
  system: service(),
  jobsController: controller('jobs'),

  isForbidden: alias('jobsController.isForbidden'),

  pendingJobs: filterBy('model', 'status', 'pending'),
  runningJobs: filterBy('model', 'status', 'running'),
  deadJobs: filterBy('model', 'status', 'dead'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['id', 'name']),

  filteredJobs: computed(
    'model.[]',
    'system.activeNamespace',
    'system.namespaces.length',
    function() {
      if (this.get('system.namespaces.length')) {
        return this.get('model').filterBy('namespace.id', this.get('system.activeNamespace.id'));
      } else {
        return this.get('model');
      }
    }
  ),

  listToSort: alias('filteredJobs'),
  listToSearch: alias('listSorted'),
  sortedJobs: alias('listSearched'),

  isShowingDeploymentDetails: false,

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job);
    },

    refresh() {
      this.send('refreshRoute');
    },
  },
});

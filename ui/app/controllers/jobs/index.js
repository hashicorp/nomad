import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

export default Controller.extend(Sortable, Searchable, {
  system: service(),
  jobsController: controller('jobs'),

  isForbidden: alias('jobsController.isForbidden'),

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
  fuzzySearchProps: computed(() => ['name']),
  fuzzySearchEnabled: true,

  /**
    Filtered jobs are those that match the selected namespace and aren't children
    of periodic or parameterized jobs.
  */
  filteredJobs: computed('model.[]', 'model.@each.parent', function() {
    // Namespace related properties are ommitted from the dependent keys
    // due to a prop invalidation bug caused by region switching.
    const hasNamespaces = this.get('system.namespaces.length');
    const activeNamespace = this.get('system.activeNamespace.id') || 'default';

    return this.get('model')
      .compact()
      .filter(job => !hasNamespaces || job.get('namespace.id') === activeNamespace)
      .filter(job => !job.get('parent.content'));
  }),

  listToSort: alias('filteredJobs'),
  listToSearch: alias('listSorted'),
  sortedJobs: alias('listSearched'),

  isShowingDeploymentDetails: false,

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job.get('plainId'));
    },
  },
});

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

  facetOptionsType: computed(() => [
    { key: 'batch', label: 'Batch' },
    { key: 'parameterized', label: 'Parameterized' },
    { key: 'periodic', label: 'Periodic' },
    { key: 'service', label: 'Service' },
    { key: 'system', label: 'System' },
  ]),

  facetOptionsStatus: computed(() => [
    { key: 'pending', label: 'Pending' },
    { key: 'running', label: 'Running' },
    { key: 'dead', label: 'Dead' },
  ]),

  facetOptionsDatacenter: computed('visibleJobs.[]', function() {
    const flatten = (acc, val) => acc.concat(val);
    const allDatacenters = new Set(
      this.get('visibleJobs')
        .mapBy('datacenters')
        .reduce(flatten, [])
    );

    return Array.from(allDatacenters)
      .compact()
      .sort()
      .map(dc => ({ key: dc, label: dc }));
  }),

  facetOptionsPrefix: computed('visibleJobs.[]', function() {
    // A prefix is defined as the start of a job name up to the first - or .
    // ex: mktg-analytics -> mktg, ds.supermodel.classifier -> ds
    const hasPrefix = /.[-._]/;

    // Collect and count all the prefixes
    const allNames = this.get('visibleJobs').mapBy('name');
    const nameHistogram = allNames.reduce((hist, name) => {
      if (hasPrefix.test(name)) {
        const prefix = name.match(/(.+?)[-.]/)[1];
        hist[prefix] = hist[prefix] ? hist[prefix] + 1 : 1;
      }
      return hist;
    }, {});

    // Convert to an array
    const nameTable = Object.keys(nameHistogram).map(key => ({
      prefix: key,
      count: nameHistogram[key],
    }));

    // Only consider prefixes that match more than one name, then convert to an
    // options array, including the counts in the label
    return nameTable
      .filter(name => name.count > 1)
      .sortBy('prefix')
      .reverse()
      .map(name => ({
        key: name.prefix,
        label: `${name.prefix} (${name.count})`,
      }));
  }),

  facetSelectionType: computed(() => []),
  facetSelectionStatus: computed(() => []),
  facetSelectionDatacenter: computed(() => []),
  facetSelectionPrefix: computed(() => []),

  /**
    Filtered jobs are those that match the selected namespace and aren't children
    of periodic or parameterized jobs.
  */
  visibleJobs: computed('model.[]', 'model.@each.parent', function() {
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

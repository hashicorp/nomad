import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';

export default Controller.extend(Sortable, Searchable, {
  system: service(),
  jobsController: controller('jobs'),

  isForbidden: alias('jobsController.isForbidden'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
    qpType: 'type',
    qpStatus: 'status',
    qpDatacenter: 'dc',
    qpPrefix: 'prefix',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['id', 'name']),
  fuzzySearchProps: computed(() => ['name']),
  fuzzySearchEnabled: true,

  qpType: '',
  qpStatus: '',
  qpDatacenter: '',
  qpPrefix: '',

  selectionType: selection('qpType'),
  selectionStatus: selection('qpStatus'),
  selectionDatacenter: selection('qpDatacenter'),
  selectionPrefix: selection('qpPrefix'),

  optionsType: computed(() => [
    { key: 'batch', label: 'Batch' },
    { key: 'parameterized', label: 'Parameterized' },
    { key: 'periodic', label: 'Periodic' },
    { key: 'service', label: 'Service' },
    { key: 'system', label: 'System' },
  ]),

  optionsStatus: computed(() => [
    { key: 'pending', label: 'Pending' },
    { key: 'running', label: 'Running' },
    { key: 'dead', label: 'Dead' },
  ]),

  optionsDatacenter: computed('visibleJobs.[]', function() {
    const flatten = (acc, val) => acc.concat(val);
    const allDatacenters = new Set(this.visibleJobs.mapBy('datacenters').reduce(flatten, []));

    // Remove any invalid datacenters from the query param/selection
    const availableDatacenters = Array.from(allDatacenters).compact();
    scheduleOnce('actions', () => {
      this.set(
        'qpDatacenter',
        serialize(intersection(availableDatacenters, this.selectionDatacenter))
      );
    });

    return availableDatacenters.sort().map(dc => ({ key: dc, label: dc }));
  }),

  optionsPrefix: computed('visibleJobs.[]', function() {
    // A prefix is defined as the start of a job name up to the first - or .
    // ex: mktg-analytics -> mktg, ds.supermodel.classifier -> ds
    const hasPrefix = /.[-._]/;

    // Collect and count all the prefixes
    const allNames = this.visibleJobs.mapBy('name');
    const nameHistogram = allNames.reduce((hist, name) => {
      if (hasPrefix.test(name)) {
        const prefix = name.match(/(.+?)[-._]/)[1];
        hist[prefix] = hist[prefix] ? hist[prefix] + 1 : 1;
      }
      return hist;
    }, {});

    // Convert to an array
    const nameTable = Object.keys(nameHistogram).map(key => ({
      prefix: key,
      count: nameHistogram[key],
    }));

    // Only consider prefixes that match more than one name
    const prefixes = nameTable.filter(name => name.count > 1);

    // Remove any invalid prefixes from the query param/selection
    const availablePrefixes = prefixes.mapBy('prefix');
    scheduleOnce('actions', () => {
      this.set('qpPrefix', serialize(intersection(availablePrefixes, this.selectionPrefix)));
    });

    // Sort, format, and include the count in the label
    return prefixes.sortBy('prefix').map(name => ({
      key: name.prefix,
      label: `${name.prefix} (${name.count})`,
    }));
  }),

  /**
    Visible jobs are those that match the selected namespace and aren't children
    of periodic or parameterized jobs.
  */
  visibleJobs: computed('model.[]', 'model.@each.parent', function() {
    // Namespace related properties are ommitted from the dependent keys
    // due to a prop invalidation bug caused by region switching.
    const hasNamespaces = this.get('system.namespaces.length');
    const activeNamespace = this.get('system.activeNamespace.id') || 'default';

    return this.model
      .compact()
      .filter(job => !hasNamespaces || job.get('namespace.id') === activeNamespace)
      .filter(job => !job.get('parent.content'));
  }),

  filteredJobs: computed(
    'visibleJobs.[]',
    'selectionType',
    'selectionStatus',
    'selectionDatacenter',
    'selectionPrefix',
    function() {
      const {
        selectionType: types,
        selectionStatus: statuses,
        selectionDatacenter: datacenters,
        selectionPrefix: prefixes,
      } = this;

      // A job must match ALL filter facets, but it can match ANY selection within a facet
      // Always return early to prevent unnecessary facet predicates.
      return this.visibleJobs.filter(job => {
        if (types.length && !types.includes(job.get('displayType'))) {
          return false;
        }

        if (statuses.length && !statuses.includes(job.get('status'))) {
          return false;
        }

        if (datacenters.length && !job.get('datacenters').find(dc => datacenters.includes(dc))) {
          return false;
        }

        const name = job.get('name');
        if (prefixes.length && !prefixes.find(prefix => name.startsWith(prefix))) {
          return false;
        }

        return true;
      });
    }
  ),

  listToSort: alias('filteredJobs'),
  listToSearch: alias('listSorted'),
  sortedJobs: alias('listSearched'),

  isShowingDeploymentDetails: false,

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  },

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job.get('plainId'));
    },
  },
});

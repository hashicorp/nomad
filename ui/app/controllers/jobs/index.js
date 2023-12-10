/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

//@ts-check

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { inject as service } from '@ember/service';
import { alias, readOnly } from '@ember/object/computed';
import Controller from '@ember/controller';
import { computed, action } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

const DEFAULT_SORT_PROPERTY = 'modifyIndex';
const DEFAULT_SORT_DESCENDING = true;

@classic
export default class IndexController extends Controller.extend(
  Sortable,
  Searchable
) {
  @service system;
  @service userSettings;
  @service router;

  isForbidden = false;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      searchTerm: 'search',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    {
      qpType: 'type',
    },
    {
      qpStatus: 'status',
    },
    {
      qpDatacenter: 'dc',
    },
    {
      qpPrefix: 'prefix',
    },
    {
      qpNamespace: 'namespace',
    },
    {
      qpNodePool: 'nodePool',
    },
  ];

  currentPage = 1;
  @readOnly('userSettings.pageSize') pageSize;

  sortProperty = DEFAULT_SORT_PROPERTY;
  sortDescending = DEFAULT_SORT_DESCENDING;

  @computed
  get searchProps() {
    return ['id', 'name'];
  }

  @computed
  get fuzzySearchProps() {
    return ['name'];
  }

  fuzzySearchEnabled = true;

  qpType = '';
  qpStatus = '';
  qpDatacenter = '';
  qpPrefix = '';
  qpNodePool = '';

  @selection('qpType') selectionType;
  @selection('qpStatus') selectionStatus;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpPrefix') selectionPrefix;
  @selection('qpNodePool') selectionNodePool;

  @computed
  get optionsType() {
    return [
      { key: 'batch', label: 'Batch' },
      { key: 'pack', label: 'Pack' },
      { key: 'parameterized', label: 'Parameterized' },
      { key: 'periodic', label: 'Periodic' },
      { key: 'service', label: 'Service' },
      { key: 'system', label: 'System' },
      { key: 'sysbatch', label: 'System Batch' },
    ];
  }

  @computed
  get optionsStatus() {
    return [
      { key: 'pending', label: 'Pending' },
      { key: 'running', label: 'Running' },
      { key: 'dead', label: 'Dead' },
    ];
  }

  @computed('selectionDatacenter', 'visibleJobs.[]')
  get optionsDatacenter() {
    const flatten = (acc, val) => acc.concat(val);
    const allDatacenters = new Set(
      this.visibleJobs.mapBy('datacenters').reduce(flatten, [])
    );

    // Remove any invalid datacenters from the query param/selection
    const availableDatacenters = Array.from(allDatacenters).compact();
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpDatacenter',
        serialize(intersection(availableDatacenters, this.selectionDatacenter))
      );
    });

    return availableDatacenters.sort().map((dc) => ({ key: dc, label: dc }));
  }

  @computed('selectionPrefix', 'visibleJobs.[]')
  get optionsPrefix() {
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
    const nameTable = Object.keys(nameHistogram).map((key) => ({
      prefix: key,
      count: nameHistogram[key],
    }));

    // Only consider prefixes that match more than one name
    const prefixes = nameTable.filter((name) => name.count > 1);

    // Remove any invalid prefixes from the query param/selection
    const availablePrefixes = prefixes.mapBy('prefix');
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpPrefix',
        serialize(intersection(availablePrefixes, this.selectionPrefix))
      );
    });

    // Sort, format, and include the count in the label
    return prefixes.sortBy('prefix').map((name) => ({
      key: name.prefix,
      label: `${name.prefix} (${name.count})`,
    }));
  }

  @computed('qpNamespace', 'model.namespaces.[]')
  get optionsNamespaces() {
    const availableNamespaces = this.model.namespaces.map((namespace) => ({
      key: namespace.name,
      label: namespace.name,
    }));

    availableNamespaces.unshift({
      key: '*',
      label: 'All (*)',
    });

    // Unset the namespace selection if it was server-side deleted
    if (!availableNamespaces.mapBy('key').includes(this.qpNamespace)) {
      scheduleOnce('actions', () => {
        // eslint-disable-next-line ember/no-side-effects
        this.set('qpNamespace', '*');
      });
    }

    return availableNamespaces;
  }

  @computed('selectionNodePool', 'model.nodePools.[]')
  get optionsNodePool() {
    const availableNodePools = this.model.nodePools;

    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpNodePool',
        serialize(
          intersection(
            availableNodePools.map(({ name }) => name),
            this.selectionNodePool
          )
        )
      );
    });

    return availableNodePools.map((nodePool) => ({
      key: nodePool.name,
      label: nodePool.name,
    }));
  }

  /**
    Visible jobs are those that match the selected namespace and aren't children
    of periodic or parameterized jobs.
  */
  @computed('model.jobs.@each.parent')
  get visibleJobs() {
    if (!this.model || !this.model.jobs) return [];
    return this.model.jobs
      .compact()
      .filter((job) => !job.isNew)
      .filter((job) => !job.get('parent.content'));
  }

  @computed(
    'visibleJobs.[]',
    'selectionType',
    'selectionStatus',
    'selectionDatacenter',
    'selectionNodePool',
    'selectionPrefix'
  )
  get filteredJobs() {
    const {
      selectionType: types,
      selectionStatus: statuses,
      selectionDatacenter: datacenters,
      selectionPrefix: prefixes,
      selectionNodePool: nodePools,
    } = this;

    // A job must match ALL filter facets, but it can match ANY selection within a facet
    // Always return early to prevent unnecessary facet predicates.
    return this.visibleJobs.filter((job) => {
      const shouldShowPack = types.includes('pack') && job.displayType.isPack;

      if (types.length && shouldShowPack) {
        return true;
      }

      if (types.length && !types.includes(job.get('displayType.type'))) {
        return false;
      }

      if (statuses.length && !statuses.includes(job.get('status'))) {
        return false;
      }

      if (
        datacenters.length &&
        !job.get('datacenters').find((dc) => datacenters.includes(dc))
      ) {
        return false;
      }

      if (nodePools.length && !nodePools.includes(job.get('nodePool'))) {
        return false;
      }

      const name = job.get('name');
      if (
        prefixes.length &&
        !prefixes.find((prefix) => name.startsWith(prefix))
      ) {
        return false;
      }

      return true;
    });
  }

  // eslint-disable-next-line ember/require-computed-property-dependencies
  @computed('searchTerm')
  get sortAtLastSearch() {
    return {
      sortProperty: this.sortProperty,
      sortDescending: this.sortDescending,
      searchTerm: this.searchTerm,
    };
  }

  @computed(
    'searchTerm',
    'sortAtLastSearch.{sortDescending,sortProperty}',
    'sortDescending',
    'sortProperty'
  )
  get prioritizeSearchOrder() {
    let shouldPrioritizeSearchOrder =
      !!this.searchTerm &&
      this.sortAtLastSearch.sortProperty === this.sortProperty &&
      this.sortAtLastSearch.sortDescending === this.sortDescending;
    if (shouldPrioritizeSearchOrder) {
      /* eslint-disable ember/no-side-effects */
      this.set('sortDescending', DEFAULT_SORT_DESCENDING);
      this.set('sortProperty', DEFAULT_SORT_PROPERTY);
      this.set('sortAtLastSearch.sortProperty', DEFAULT_SORT_PROPERTY);
      this.set('sortAtLastSearch.sortDescending', DEFAULT_SORT_DESCENDING);
    }
    /* eslint-enable ember/no-side-effects */
    return shouldPrioritizeSearchOrder;
  }

  @alias('filteredJobs') listToSearch;
  @alias('listSearched') listToSort;

  // sortedJobs is what we use to populate the table;
  // If the user has searched but not sorted, we return the (fuzzy) searched list verbatim
  // If the user has sorted, we allow the fuzzy search to filter down the list, but return it in a sorted order.
  get sortedJobs() {
    return this.prioritizeSearchOrder ? this.listSearched : this.listSorted;
  }

  isShowingDeploymentDetails = false;

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @action
  goToRun() {
    this.router.transitionTo('jobs.run');
  }
}

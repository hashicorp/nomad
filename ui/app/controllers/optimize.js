/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { scheduleOnce } from '@ember/runloop';
import { task } from 'ember-concurrency';
import intersection from 'lodash.intersection';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';

import EmberObject, { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import Searchable from 'nomad-ui/mixins/searchable';
import classic from 'ember-classic-decorator';

export default class OptimizeController extends Controller {
  @controller('optimize/summary') summaryController;
  @service router;
  @service system;

  queryParams = [
    {
      searchTerm: 'search',
    },
    {
      qpNamespace: 'namespacefilter',
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
  ];

  constructor() {
    super(...arguments);

    this.summarySearch = RecommendationSummarySearch.create({
      dataSource: this,
    });
  }

  get namespaces() {
    return this.model.namespaces;
  }

  get summaries() {
    return this.model.summaries;
  }

  @tracked searchTerm = '';

  @tracked qpType = '';
  @tracked qpStatus = '';
  @tracked qpDatacenter = '';
  @tracked qpPrefix = '';
  @tracked qpNamespace = '*';

  @selection('qpType') selectionType;
  @selection('qpStatus') selectionStatus;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpPrefix') selectionPrefix;

  get optionsNamespaces() {
    const availableNamespaces = this.namespaces.map((namespace) => ({
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
        this.qpNamespace = '*';
      });
    }

    return availableNamespaces;
  }

  optionsType = [
    { key: 'service', label: 'Service' },
    { key: 'system', label: 'System' },
  ];

  optionsStatus = [
    { key: 'pending', label: 'Pending' },
    { key: 'running', label: 'Running' },
    { key: 'dead', label: 'Dead' },
  ];

  get optionsDatacenter() {
    const flatten = (acc, val) => acc.concat(val);
    const allDatacenters = new Set(
      this.summaries.mapBy('job.datacenters').reduce(flatten, [])
    );

    // Remove any invalid datacenters from the query param/selection
    const availableDatacenters = Array.from(allDatacenters).compact();
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.qpDatacenter = serialize(
        intersection(availableDatacenters, this.selectionDatacenter)
      );
    });

    return availableDatacenters.sort().map((dc) => ({ key: dc, label: dc }));
  }

  get optionsPrefix() {
    // A prefix is defined as the start of a job name up to the first - or .
    // ex: mktg-analytics -> mktg, ds.supermodel.classifier -> ds
    const hasPrefix = /.[-._]/;

    // Collect and count all the prefixes
    const allNames = this.summaries.mapBy('job.name');
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
      this.qpPrefix = serialize(
        intersection(availablePrefixes, this.selectionPrefix)
      );
    });

    // Sort, format, and include the count in the label
    return prefixes.sortBy('prefix').map((name) => ({
      key: name.prefix,
      label: `${name.prefix} (${name.count})`,
    }));
  }

  get filteredSummaries() {
    const {
      selectionType: types,
      selectionStatus: statuses,
      selectionDatacenter: datacenters,
      selectionPrefix: prefixes,
    } = this;

    // A summary’s job must match ALL filter facets, but it can match ANY selection within a facet
    // Always return early to prevent unnecessary facet predicates.
    return this.summarySearch.listSearched.filter((summary) => {
      const job = summary.get('job');

      if (job.isDestroying) {
        return false;
      }

      if (
        this.qpNamespace !== '*' &&
        job.get('namespace.name') !== this.qpNamespace
      ) {
        return false;
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

  get activeRecommendationSummary() {
    if (this.router.currentRouteName === 'optimize.summary') {
      return this.summaryController.model;
    } else {
      return undefined;
    }
  }

  // This is a task because the accordion uses timeouts for animation
  // eslint-disable-next-line require-yield
  @(task(function* () {
    const currentSummaryIndex = this.filteredSummaries.indexOf(
      this.activeRecommendationSummary
    );
    const nextSummary = this.filteredSummaries.objectAt(
      currentSummaryIndex + 1
    );

    if (nextSummary) {
      this.transitionToSummary(nextSummary);
    } else {
      this.send('reachedEnd');
    }
  }).drop())
  proceed;

  @action
  transitionToSummary(summary) {
    this.transitionToRoute('optimize.summary', summary.slug, {
      queryParams: { jobNamespace: summary.jobNamespace },
    });
  }

  @action
  setFacetQueryParam(queryParam, selection) {
    this[queryParam] = serialize(selection);
    this.syncActiveSummary();
  }

  @action
  syncActiveSummary() {
    scheduleOnce('actions', () => {
      if (
        !this.activeRecommendationSummary ||
        !this.filteredSummaries.includes(this.activeRecommendationSummary)
      ) {
        const firstFilteredSummary = this.filteredSummaries.objectAt(0);

        if (firstFilteredSummary) {
          this.transitionToSummary(firstFilteredSummary);
        } else {
          this.transitionToRoute('optimize');
        }
      }
    });
  }
}

@classic
class RecommendationSummarySearch extends EmberObject.extend(Searchable) {
  @computed
  get fuzzySearchProps() {
    return ['slug'];
  }

  @alias('dataSource.summaries') listToSearch;
  @alias('dataSource.searchTerm') searchTerm;

  exactMatchEnabled = false;
  fuzzySearchEnabled = true;
  includeFuzzySearchMatches = true;
}

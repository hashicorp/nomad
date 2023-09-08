/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class AllocationsController extends Controller.extend(
  SortableFactory(['updateTime', 'healthy'])
) {
  @service userSettings;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    {
      qpHealth: 'healthy',
    },
    {
      qpType: 'type',
    },
  ];

  currentPage = 1;
  @readOnly('userSettings.pageSize') pageSize;

  sortProperty = 'updateTime';
  sortDescending = false;

  qpType = '';
  qpHealth = '';

  @selection('qpType') selectionType;
  @selection('qpHealth') selectionHealth;

  @computed
  get optionsType() {
    return [
      { key: 'controller', label: 'Controller' },
      { key: 'node', label: 'Node' },
    ];
  }

  @computed
  get optionsHealth() {
    return [
      { key: 'true', label: 'Healthy' },
      { key: 'false', label: 'Unhealthy' },
    ];
  }

  @computed('model.{controllers.[],nodes.[]}')
  get combinedAllocations() {
    return this.model.controllers.toArray().concat(this.model.nodes.toArray());
  }

  @computed(
    'combinedAllocations.[]',
    'model.{controllers.[],nodes.[]}',
    'selectionType',
    'selectionHealth'
  )
  get filteredAllocations() {
    const { selectionType: types, selectionHealth: healths } = this;

    // Instead of filtering the combined list, revert back to one of the two
    // pre-existing lists.
    let listToFilter = this.combinedAllocations;
    if (types.length === 1 && types[0] === 'controller') {
      listToFilter = this.model.controllers;
    } else if (types.length === 1 && types[0] === 'node') {
      listToFilter = this.model.nodes;
    }

    if (healths.length === 1 && healths[0] === 'true')
      return listToFilter.filterBy('healthy');
    if (healths.length === 1 && healths[0] === 'false')
      return listToFilter.filterBy('healthy', false);
    return listToFilter;
  }

  @alias('filteredAllocations') listToSort;
  @alias('listSorted') sortedAllocations;

  resetPagination() {
    if (this.currentPage != null) {
      this.set('currentPage', 1);
    }
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @action
  gotoAllocation(allocation, event) {
    lazyClick([
      () => this.transitionToRoute('allocations.allocation', allocation.id),
      event,
    ]);
  }
}

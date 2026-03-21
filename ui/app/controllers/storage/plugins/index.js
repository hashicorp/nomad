/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { service } from '@ember/service';
import { action, computed } from '@ember/object';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class IndexController extends Controller.extend(
  SortableFactory([
    'plainId',
    'controllersHealthyProportion',
    'nodesHealthyProportion',
    'provider',
  ]),
  Searchable,
) {
  @service userSettings;
  @service router;
  @controller('storage/plugins') pluginsController;

  get isForbidden() {
    return this.pluginsController.isForbidden;
  }

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
  ];

  currentPage = 1;

  get pageSize() {
    return this.userSettings.pageSize;
  }

  @computed
  get searchProps() {
    return ['id'];
  }

  @computed
  get fuzzySearchProps() {
    return ['id'];
  }

  sortProperty = 'id';
  sortDescending = false;

  get listToSort() {
    return this.model;
  }

  get listToSearch() {
    return this.listSorted;
  }

  get sortedPlugins() {
    return this.listSearched;
  }

  @action
  gotoPlugin(plugin, event) {
    lazyClick([
      () => this.router.transitionTo('storage.plugins.plugin', plugin.plainId),
      event,
    ]);
  }

  @action
  updateSearchTerm(searchTerm) {
    this.searchTerm = searchTerm;
    this.resetPagination();
  }
}

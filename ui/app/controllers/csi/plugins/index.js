/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(
  SortableFactory([
    'plainId',
    'controllersHealthyProportion',
    'nodesHealthyProportion',
    'provider',
  ]),
  Searchable
) {
  @service userSettings;
  @controller('csi/plugins') pluginsController;

  @alias('pluginsController.isForbidden') isForbidden;

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
  @readOnly('userSettings.pageSize') pageSize;

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

  @alias('model') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedPlugins;

  @action
  gotoPlugin(plugin, event) {
    lazyClick([
      () => this.transitionToRoute('csi.plugins.plugin', plugin.plainId),
      event,
    ]);
  }
}

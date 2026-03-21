/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';

export default class IndexController extends Controller.extend(
  SortableFactory(['isLeader', 'name']),
) {
  @controller('servers') serversController;

  get isForbidden() {
    return this.serversController.isForbidden;
  }

  get nodes() {
    return this.model.nodes;
  }

  get agents() {
    return this.model.agents;
  }

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
  ];

  currentPage = 1;
  pageSize = 8;

  sortProperty = 'isLeader';
  sortDescending = true;

  get listToSort() {
    return this.agents;
  }

  get sortedAgents() {
    return this.listSorted;
  }
}

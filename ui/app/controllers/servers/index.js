/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';

export default class IndexController extends Controller.extend(
  SortableFactory(['isLeader', 'name']),
) {
  @controller('servers') serversController;
  @alias('serversController.isForbidden') isForbidden;

  @alias('model.nodes') nodes;
  @alias('model.agents') agents;

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

  @alias('agents') listToSort;
  @alias('listSorted') sortedAgents;
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import Sortable from 'nomad-ui/mixins/sortable';

export default class IndexController extends Controller.extend(Sortable) {
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

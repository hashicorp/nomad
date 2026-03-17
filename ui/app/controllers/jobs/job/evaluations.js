/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import classic from 'ember-classic-decorator';

@classic
export default class EvaluationsController extends Controller.extend(
  WithNamespaceResetting,
  SortableFactory(['modifyIndex']),
) {
  queryParams = [
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @alias('model') job;
  @alias('model.evaluations') evaluations;

  @alias('evaluations') listToSort;
  @alias('listSorted') sortedEvaluations;
}

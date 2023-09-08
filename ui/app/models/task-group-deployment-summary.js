/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { gt, alias } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class TaskGroupDeploymentSummary extends Fragment {
  @fragmentOwner() deployment;

  @attr('string') name;

  @attr('boolean') autoRevert;
  @attr('boolean') autoPromote;
  @attr('boolean') promoted;
  @gt('desiredCanaries', 0) requiresPromotion;

  // The list of canary allocation IDs
  // hasMany is not supported in fragments
  @attr({ defaultValue: () => [] }) placedCanaryAllocations;

  @alias('placedCanaryAllocations.length') placedCanaries;
  @attr('number') desiredCanaries;
  @attr('number') desiredTotal;
  @attr('number') placedAllocs;
  @attr('number') healthyAllocs;
  @attr('number') unhealthyAllocs;

  @attr('date') requireProgressBy;
}

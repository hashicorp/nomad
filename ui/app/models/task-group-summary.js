/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { sum, collect } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class TaskGroupSummary extends Fragment {
  @fragmentOwner() job;
  @attr('string') name;

  @attr('number') queuedAllocs;
  @attr('number') startingAllocs;
  @attr('number') runningAllocs;
  @attr('number') completeAllocs;
  @attr('number') failedAllocs;
  @attr('number') lostAllocs;
  @attr('number') unknownAllocs;

  @collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs',
    'unknownAllocs'
  )
  allocsList;

  @sum('allocsList') totalAllocs;
}

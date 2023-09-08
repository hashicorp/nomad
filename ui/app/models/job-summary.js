/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { collect, sum } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';
import classic from 'ember-classic-decorator';

@classic
export default class JobSummary extends Model {
  @belongsTo('job') job;

  @fragmentArray('task-group-summary') taskGroupSummaries;

  // Aggregate allocation counts across all summaries
  @sumAggregation('taskGroupSummaries', 'queuedAllocs') queuedAllocs;
  @sumAggregation('taskGroupSummaries', 'startingAllocs') startingAllocs;
  @sumAggregation('taskGroupSummaries', 'runningAllocs') runningAllocs;
  @sumAggregation('taskGroupSummaries', 'completeAllocs') completeAllocs;
  @sumAggregation('taskGroupSummaries', 'failedAllocs') failedAllocs;
  @sumAggregation('taskGroupSummaries', 'unknownAllocs') unknownAllocs;
  @sumAggregation('taskGroupSummaries', 'lostAllocs') lostAllocs;

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

  @attr('number') pendingChildren;
  @attr('number') runningChildren;
  @attr('number') deadChildren;

  @collect('pendingChildren', 'runningChildren', 'deadChildren') childrenList;

  @sum('childrenList') totalChildren;
}

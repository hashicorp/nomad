/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default class EvaluationStub extends Model {
  @shortUUIDProperty('id') shortId;
  @attr('number') priority;
  @attr('string') type;
  @attr('string') triggeredBy;
  @attr('string') namespace;
  @attr('string') jobId;
  @attr('string') nodeId;
  @attr('string') deploymentId;
  @attr('string') status;
  @attr('string') statusDescription;
  @attr('date') waitUntil;
  @attr('string') previousEval;
  @attr('string') nextEval;
  @attr('string') blockedEval;
  @attr('number') modifyIndex;
  @attr('date') modifyTime;
  @attr('number') createIndex;
  @attr('date') createTime;
}

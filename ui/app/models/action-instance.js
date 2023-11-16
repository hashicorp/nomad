/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';

export default class ActionInstanceModel extends Model {
  @belongsTo('action') action;

  /**
   * @type {'starting'|'running'|'complete'}
   */
  @attr('string') state;

  @attr('string', {
    defaultValue() {
      return '';
    },
  })
  messages;
  @attr('date') createdAt;

  @attr('date') completedAt;

  @attr('string') allocID;

  /**
   * @type {WebSocket}
   */
  @attr() socket;
}

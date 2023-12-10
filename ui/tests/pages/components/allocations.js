/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  collection,
  clickable,
  isPresent,
  text,
} from 'ember-cli-page-object';
import { singularize } from 'ember-inflector';

export default function (
  selector = '[data-test-allocation]',
  propKey = 'allocations'
) {
  const lookupKey = `${singularize(propKey)}For`;
  // Remove the bracket notation
  const attr = selector.substring(1, selector.length - 1);

  return {
    [propKey]: collection(selector, {
      id: attribute(attr),
      shortId: text('[data-test-short-id]'),
      createTime: text('[data-test-create-time]'),
      createTooltip: attribute(
        'aria-label',
        '[data-test-create-time] .tooltip'
      ),
      modifyTime: text('[data-test-modify-time]'),
      health: text('[data-test-health]'),
      status: text('[data-test-client-status]'),
      job: text('[data-test-job]'),
      taskGroup: text('[data-test-task-group]'),
      client: text('[data-test-client]'),
      clientTooltip: attribute('aria-label', '[data-test-client] .tooltip'),
      jobVersion: text('[data-test-job-version]'),
      volume: text('[data-test-volume]'),
      cpu: text('[data-test-cpu]'),
      cpuTooltip: attribute('aria-label', '[data-test-cpu] .tooltip'),
      mem: text('[data-test-mem]'),
      memTooltip: attribute('aria-label', '[data-test-mem] .tooltip'),
      rescheduled: isPresent(
        '[data-test-indicators] [data-test-icon="reschedule"]'
      ),

      visit: clickable('[data-test-short-id] a'),
      visitRow: clickable(),
      visitJob: clickable('[data-test-job]'),
      visitClient: clickable('[data-test-client] a'),
    }),

    [lookupKey]: function (id) {
      return this[propKey].toArray().find((allocation) => allocation.id === id);
    },
  };
}

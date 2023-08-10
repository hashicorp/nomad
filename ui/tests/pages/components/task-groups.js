/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { collection, clickable, text } from 'ember-cli-page-object';
import { singularize } from 'ember-inflector';

export default function (
  selector = '[data-test-task-group]',
  propKey = 'taskGroups'
) {
  const lookupKey = `${singularize(propKey)}For`;

  return {
    [propKey]: collection(selector, {
      name: text('[data-test-task-group-name]'),
      count: text('[data-test-task-group-count]'),
      volume: text('[data-test-task-group-volume]'),
      cpu: text('[data-test-task-group-cpu]'),
      mem: text('[data-test-task-group-mem]'),
      disk: text('[data-test-task-group-disk]'),
      visit: clickable('[data-test-task-group-name] a'),
      visitRow: clickable(),
    }),

    [lookupKey]: function (name) {
      return this[propKey].toArray().find((tg) => tg.name === name);
    },
  };
}

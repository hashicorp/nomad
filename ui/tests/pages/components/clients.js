/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attribute, collection, clickable, text } from 'ember-cli-page-object';
import { singularize } from 'ember-inflector';

export default function (selector = '[data-test-client]', propKey = 'clients') {
  const lookupKey = `${singularize(propKey)}For`;
  // Remove the bracket notation
  const attr = selector.substring(1, selector.length - 1);

  return {
    [propKey]: collection(selector, {
      id: attribute(attr),
      shortId: text('[data-test-short-id]'),
      name: text('[data-test-name]'),
      status: text('[data-test-job-status]'),

      createTime: {
        scope: '[data-test-create-time]',
        tooltip: {
          scope: '.tooltip',
          text: attribute('aria-label'),
        },
      },

      modifyTime: {
        scope: '[data-test-modify-time]',
        tooltip: {
          scope: '.tooltip',
          text: attribute('aria-label'),
        },
      },

      visit: clickable('[data-test-short-id] a'),
      visitRow: clickable(),
    }),

    [lookupKey]: function (id) {
      return this[propKey].toArray().find((client) => client.id === id);
    },
  };
}

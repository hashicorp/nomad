/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  create,
  collection,
  clickable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/servers/:name'),

  title: text('[data-test-title]'),
  serverStatus: text('[data-test-status]'),
  address: text('[data-test-address]'),
  datacenter: text('[data-test-datacenter]'),
  hasLeaderBadge: isPresent('[data-test-leader-badge]'),

  tags: collection('[data-test-server-tag]', {
    name: text('td', { at: 0 }),
    value: text('td', { at: 1 }),
  }),

  error: {
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});

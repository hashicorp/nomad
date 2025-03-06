/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, isPresent, text, visitable } from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';

export default create({
  visit: visitable('/storage/volumes/dynamic/:id'),

  title: text('[data-test-title]'),

  // health: text('[data-test-volume-health]'),
  // provider: text('[data-test-volume-provider]'),
  node: text('[data-test-volume-node]'),
  plugin: text('[data-test-volume-plugin]'),
  hasNamespace: isPresent('[data-test-volume-namespace]'),
  namespace: text('[data-test-volume-namespace]'),

  ...allocations('[data-test-allocation]', 'allocations'),

  allocationsTableIsEmpty: isPresent('[data-test-empty-allocations]'),
  allocationsEmptyState: {
    headline: text('[data-test-empty-allocations-headline]'),
  },

  capabilities: {
    accessMode: text('[data-test-access-mode]'),
    attachmentMode: text('[data-test-attachment-mode]'),
  },
});

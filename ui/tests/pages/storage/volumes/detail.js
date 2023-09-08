/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, isPresent, text, visitable } from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';

export default create({
  visit: visitable('/csi/volumes/:id'),

  title: text('[data-test-title]'),

  health: text('[data-test-volume-health]'),
  provider: text('[data-test-volume-provider]'),
  externalId: text('[data-test-volume-external-id]'),
  hasNamespace: isPresent('[data-test-volume-namespace]'),
  namespace: text('[data-test-volume-namespace]'),

  ...allocations('[data-test-read-allocation]', 'readAllocations'),
  ...allocations('[data-test-write-allocation]', 'writeAllocations'),

  writeTableIsEmpty: isPresent('[data-test-empty-write-allocations]'),
  writeEmptyState: {
    headline: text('[data-test-empty-write-allocations-headline]'),
  },

  readTableIsEmpty: isPresent('[data-test-empty-read-allocations]'),
  readEmptyState: {
    headline: text('[data-test-empty-read-allocations-headline]'),
  },

  constraints: {
    accessMode: text('[data-test-access-mode]'),
    attachmentMode: text('[data-test-attachment-mode]'),
  },
});

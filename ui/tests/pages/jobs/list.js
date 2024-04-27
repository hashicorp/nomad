/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  create,
  collection,
  clickable,
  isPresent,
  text,
  triggerable,
  visitable,
} from 'ember-cli-page-object';

import { hdsFacet } from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/jobs'),

  search: {
    scope: '[data-test-jobs-search]',
    keydown: triggerable('keydown'),
  },

  runJobButton: {
    scope: '[data-test-run-job]',
    isDisabled: attribute('disabled'),
  },

  jobs: collection('[data-test-job-row]', {
    id: attribute('data-test-job-row'),
    name: text('[data-test-job-name]'),
    link: attribute('href', '[data-test-job-name] a'),
    namespace: text('[data-test-job-namespace]'),
    nodePool: text('[data-test-job-node-pool]'),
    status: text('[data-test-job-status]'),
    type: text('[data-test-job-type]'),

    hasNamespace: isPresent('[data-test-job-namespace]'),
    clickRow: clickable(),
    clickName: clickable('[data-test-job-name] a'),
  }),

  isEmpty: isPresent('[data-test-empty-jobs-list]'),
  emptyState: {
    headline: text('[data-test-empty-jobs-list-headline]'),
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
    gotoJobs: clickable('[data-test-error-jobs-link]'),
    gotoClients: clickable('[data-test-error-clients-link]'),
    gotoSignin: clickable('[data-test-error-signin-link]'),
  },

  pageSizeSelect: pageSizeSelect(),

  facets: {
    namespace: hdsFacet('[data-test-facet="Namespace"]'),
    type: hdsFacet('[data-test-facet="Type"]'),
    status: hdsFacet('[data-test-facet="Status"]'),
    nodePool: hdsFacet('[data-test-facet="NodePool"]'),
  },
});

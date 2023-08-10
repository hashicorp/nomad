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
  property,
  text,
  triggerable,
  visitable,
} from 'ember-cli-page-object';

import { multiFacet, singleFacet } from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/jobs'),

  search: {
    scope: '[data-test-jobs-search] input',
    keydown: triggerable('keydown'),
  },

  runJobButton: {
    scope: '[data-test-run-job]',
    isDisabled: property('disabled'),
  },

  jobs: collection('[data-test-job-row]', {
    id: attribute('data-test-job-row'),
    name: text('[data-test-job-name]'),
    link: attribute('href', '[data-test-job-name] a'),
    namespace: text('[data-test-job-namespace]'),
    nodePool: text('[data-test-job-node-pool]'),
    status: text('[data-test-job-status]'),
    type: text('[data-test-job-type]'),
    priority: text('[data-test-job-priority]'),
    taskGroups: text('[data-test-job-task-groups]'),

    hasNamespace: isPresent('[data-test-job-namespace]'),
    clickRow: clickable(),
    clickName: clickable('[data-test-job-name] a'),
  }),

  nextPage: clickable('[data-test-pager="next"]'),
  prevPage: clickable('[data-test-pager="prev"]'),

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
    namespace: singleFacet('[data-test-namespace-facet]'),
    type: multiFacet('[data-test-type-facet]'),
    status: multiFacet('[data-test-status-facet]'),
    datacenter: multiFacet('[data-test-datacenter-facet]'),
    prefix: multiFacet('[data-test-prefix-facet]'),
  },
});

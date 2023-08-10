/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  collection,
  create,
  hasClass,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import recommendationCard from 'nomad-ui/tests/pages/components/recommendation-card';
import { multiFacet, singleFacet } from 'nomad-ui/tests/pages/components/facet';

export default create({
  visit: visitable('/optimize'),

  search: {
    scope: '[data-test-recommendation-summaries-search] input',
    placeholder: attribute('placeholder'),
  },

  facets: {
    namespace: singleFacet('[data-test-namespace-facet]'),
    type: multiFacet('[data-test-type-facet]'),
    status: multiFacet('[data-test-status-facet]'),
    datacenter: multiFacet('[data-test-datacenter-facet]'),
    prefix: multiFacet('[data-test-prefix-facet]'),
  },

  card: recommendationCard,

  recommendationSummaries: collection(
    '[data-test-recommendation-summary-row]',
    {
      isActive: hasClass('is-active'),
      isDisabled: hasClass('is-disabled'),

      slug: text('[data-test-slug]'),
      namespace: text('[data-test-namespace]'),
      date: text('[data-test-date]'),
      allocationCount: text('[data-test-allocation-count]'),
      cpu: text('[data-test-cpu]'),
      memory: text('[data-test-memory]'),
      aggregateCpu: text('[data-test-aggregate-cpu]'),
      aggregateMemory: text('[data-test-aggregate-memory]'),
    }
  ),

  empty: {
    scope: '[data-test-empty-recommendations]',
    headline: text('[data-test-empty-recommendations-headline]'),
  },

  error: {
    scope: '[data-test-recommendation-error]',
    headline: text('[data-test-headline]'),
    errors: text('[data-test-errors]'),
    dismiss: clickable('[data-test-dismiss]'),
  },

  applicationError: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
  },
});

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
import facet from 'nomad-ui/tests/pages/components/facet';
import toggle from 'nomad-ui/tests/pages/components/toggle';

export default create({
  visit: visitable('/optimize'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  search: {
    scope: '[data-test-recommendation-summaries-search] input',
    placeholder: attribute('placeholder'),
  },

  facets: {
    type: facet('[data-test-type-facet]'),
    status: facet('[data-test-status-facet]'),
    datacenter: facet('[data-test-datacenter-facet]'),
    prefix: facet('[data-test-prefix-facet]'),
  },

  allNamespacesToggle: toggle('[data-test-all-namespaces-toggle]'),

  card: recommendationCard,

  recommendationSummaries: collection('[data-test-recommendation-summary-row]', {
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
  }),

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

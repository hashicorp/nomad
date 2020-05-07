import {
  attribute,
  clickable,
  collection,
  create,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';

export default create({
  visit: visitable('/csi/plugins/:id'),

  title: text('[data-test-title]'),

  controllerHealth: text('[data-test-plugin-controller-health]'),
  nodeHealth: text('[data-test-plugin-node-health]'),
  provider: text('[data-test-plugin-provider]'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  ...allocations('[data-test-controller-allocation]', 'controllerAllocations'),
  ...allocations('[data-test-node-allocation]', 'nodeAllocations'),

  controllerTableIsEmpty: isPresent('[data-test-empty-controller-allocations]'),
  controllerEmptyState: {
    headline: text('[data-test-empty-controller-allocations-headline]'),
  },

  nodeTableIsEmpty: isPresent('[data-test-empty-node-allocations]'),
  nodeEmptyState: {
    headline: text('[data-test-empty-node-allocations-headline]'),
  },
});

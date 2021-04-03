import {
  clickable,
  collection,
  create,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import error from 'nomad-ui/tests/pages/components/error';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/csi/volumes'),

  search: fillable('[data-test-volumes-search] input'),

  volumes: collection('[data-test-volume-row]', {
    name: text('[data-test-volume-name]'),
    schedulable: text('[data-test-volume-schedulable]'),
    controllerHealth: text('[data-test-volume-controller-health]'),
    nodeHealth: text('[data-test-volume-node-health]'),
    provider: text('[data-test-volume-provider]'),
    allocations: text('[data-test-volume-allocations]'),

    clickRow: clickable(),
    clickName: clickable('[data-test-volume-name] a'),
  }),

  nextPage: clickable('[data-test-pager="next"]'),
  prevPage: clickable('[data-test-pager="prev"]'),

  isEmpty: isPresent('[data-test-empty-volumes-list]'),
  emptyState: {
    headline: text('[data-test-empty-volumes-list-headline]'),
  },

  error: error(),
  pageSizeSelect: pageSizeSelect(),

  namespaceSwitcher: {
    isPresent: isPresent('[data-test-namespace-switcher-parent]'),
    open: clickable('[data-test-namespace-switcher-parent] .ember-power-select-trigger'),
    options: collection('.ember-power-select-option', {
      testContainer: '#ember-testing',
      resetScope: true,
      label: text(),
    }),
  },
});

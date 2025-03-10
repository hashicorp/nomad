/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
import { hdsFacet } from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/storage'),

  csiSearch: fillable('[data-test-csi-volumes-search]'),
  dynamicHostVolumesSearch: fillable(
    '[data-test-dynamic-host-volumes-search] input'
  ),
  staticHostVolumesSearch: fillable(
    '[data-test-static-host-volumes-search] input'
  ),
  ephemeralDisksSearch: fillable('[data-test-ephemeral-disks-search] input'),

  csiVolumes: collection('[data-test-csi-volume-row]', {
    name: text('[data-test-csi-volume-name]'),
    namespace: text('[data-test-csi-volume-namespace]'),
    schedulable: text('[data-test-csi-volume-schedulable]'),
    controllerHealth: text('[data-test-csi-volume-controller-health]'),
    nodeHealth: text('[data-test-csi-volume-node-health]'),
    plugin: text('[data-test-csi-volume-plugin]'),
    allocations: text('[data-test-csi-volume-allocations]'),

    hasNamespace: isPresent('[data-test-csi-volume-namespace]'),
    clickRow: clickable(),
    clickName: clickable('[data-test-csi-volume-name] a'),
  }),

  csiIsEmpty: isPresent('[data-test-empty-csi-volumes-list-headline]'),
  csiEmptyState: text('[data-test-empty-csi-volumes-list-headline]'),

  csiNextPage: clickable('.hds-pagination-nav__arrow--direction-next'),
  csiPrevPage: clickable('.hds-pagination-nav__arrow--direction-prev'),

  error: error(),
  pageSizeSelect: pageSizeSelect(),

  facets: {
    namespace: hdsFacet('[data-test-namespace-facet]'),
  },
});

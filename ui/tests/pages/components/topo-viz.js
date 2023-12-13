/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { collection, isPresent } from 'ember-cli-page-object';
import TopoVizDatacenter from './topo-viz/datacenter';

export default (scope) => ({
  scope,

  datacenters: collection(
    '[data-test-topo-viz-datacenter]',
    TopoVizDatacenter()
  ),

  allocationAssociationsArePresent: isPresent(
    '[data-test-allocation-associations]'
  ),
  allocationAssociations: collection('[data-test-allocation-association]'),
});

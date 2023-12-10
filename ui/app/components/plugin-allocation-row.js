/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AllocationRow from 'nomad-ui/components/allocation-row';
import classic from 'ember-classic-decorator';
import { attributeBindings } from '@ember-decorators/component';

@classic
@attributeBindings(
  'data-test-controller-allocation',
  'data-test-node-allocation'
)
export default class PluginAllocationRow extends AllocationRow {
  pluginAllocation = null;
  allocation = null;

  didReceiveAttrs() {
    // Allocation is always set through pluginAllocation
    this.set('allocation', null);
    this.setAllocation();
  }

  // The allocation for the plugin's controller or storage plugin needs
  // to be imperatively fetched since these plugins are Fragments which
  // can't have relationships.
  async setAllocation() {
    if (this.pluginAllocation && !this.allocation) {
      const allocation = await this.pluginAllocation.getAllocation();
      if (!this.isDestroyed) {
        this.set('allocation', allocation);
        this.updateStatsTracker();
      }
    }
  }
}

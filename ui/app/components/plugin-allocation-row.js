import AllocationRow from 'nomad-ui/components/allocation-row';
import classic from 'ember-classic-decorator';

@classic
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

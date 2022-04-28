import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';

export default class VolumeController extends Controller {
  // Used in the template
  @service system;

  queryParams = [
    {
      volumeNamespace: 'namespace',
    },
  ];
  volumeNamespace = 'default';

  @computed('model.readAllocations.@each.modifyIndex')
  get sortedReadAllocations() {
    return this.model.readAllocations.sortBy('modifyIndex').reverse();
  }

  @computed('model.writeAllocations.@each.modifyIndex')
  get sortedWriteAllocations() {
    return this.model.writeAllocations.sortBy('modifyIndex').reverse();
  }

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation);
  }
}

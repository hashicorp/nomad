/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';

export default class DynamicHostVolumeController extends Controller {
  // Used in the template
  @service system;
  @service router;

  queryParams = [
    {
      namespace: 'namespace',
    },
  ];
  namespace = 'default';

  get volume() {
    return this.model;
  }

  get breadcrumbs() {
    const volume = this.volume;
    if (!volume) {
      return [];
    }
    return [
      {
        label: 'Storage',
        args: ['storage.index'],
      },
      {
        label: 'Volumes',
        args: ['storage.volumes'],
      },
      {
        label: volume.name,
        args: ['storage.volumes.dynamic-host-volume', volume.idWithNamespace],
      },
    ];
  }

  @computed('model.allocations.@each.modifyIndex')
  get sortedAllocations() {
    const allocations = this.model?.allocations;

    if (!allocations) {
      return [];
    }

    return allocations.sortBy('modifyIndex').reverse();
  }

  @action gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }
}

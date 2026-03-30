/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';
import { action, computed } from '@ember/object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default class VolumeController extends Controller {
  // Used in the template
  @service system;
  @service router;

  queryParams = [
    {
      volumeNamespace: 'namespace',
    },
  ];
  volumeNamespace = 'default';

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
        label: volume.name,
        args: [
          'storage.volumes.volume',
          volume.plainId,
          qpBuilder({
            volumeNamespace: volume.get('namespace.name') || 'default',
          }),
        ],
      },
    ];
  }

  @computed('model.readAllocations.@each.modifyIndex')
  get sortedReadAllocations() {
    return [...this.model.readAllocations].sort((a, b) => (b.modifyIndex || 0) - (a.modifyIndex || 0));
  }

  @computed('model.writeAllocations.@each.modifyIndex')
  get sortedWriteAllocations() {
    return [...this.model.writeAllocations].sort((a, b) => (b.modifyIndex || 0) - (a.modifyIndex || 0));
  }

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default class VolumeController extends Controller {
  // Used in the template
  @service system;

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
    return [
      {
        label: 'Volumes',
        args: [
          'csi.volumes',
          qpBuilder({
            volumeNamespace: volume.get('namespace.name') || 'default',
          }),
        ],
      },
      {
        label: volume.name,
        args: [
          'csi.volumes.volume',
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
    return this.model.readAllocations.sortBy('modifyIndex').reverse();
  }

  @computed('model.writeAllocations.@each.modifyIndex')
  get sortedWriteAllocations() {
    return this.model.writeAllocations.sortBy('modifyIndex').reverse();
  }

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
  }
}
